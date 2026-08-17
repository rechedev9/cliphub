if (-not ("ClipHub.BuildPublicationFiles" -as [type])) {
    Add-Type -TypeDefinition @"
using System;
using System.ComponentModel;
using System.IO;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

namespace ClipHub {
    public static class BuildPublicationFiles {
        private const uint GENERIC_READ = 0x80000000;
        private const uint GENERIC_WRITE = 0x40000000;
        private const uint FILE_SHARE_READ = 0x00000001;
        private const uint FILE_SHARE_WRITE = 0x00000002;
        private const uint FILE_SHARE_DELETE = 0x00000004;
        private const uint OPEN_EXISTING = 3;
        private const uint OPEN_ALWAYS = 4;
        private const uint FILE_ATTRIBUTE_DIRECTORY = 0x00000010;
        private const uint FILE_ATTRIBUTE_REPARSE_POINT = 0x00000400;
        private const uint FILE_ATTRIBUTE_NORMAL = 0x00000080;
        private const uint FILE_FLAG_BACKUP_SEMANTICS = 0x02000000;
        private const uint FILE_FLAG_OPEN_REPARSE_POINT = 0x00200000;
        private const uint FILE_TYPE_DISK = 0x0001;
        private const uint MOVEFILE_REPLACE_EXISTING = 0x00000001;
        private const uint MOVEFILE_WRITE_THROUGH = 0x00000008;
        private const int ERROR_FILE_NOT_FOUND = 2;
        private const int ERROR_PATH_NOT_FOUND = 3;

        [StructLayout(LayoutKind.Sequential)]
        private struct BY_HANDLE_FILE_INFORMATION {
            public uint FileAttributes;
            public System.Runtime.InteropServices.ComTypes.FILETIME CreationTime;
            public System.Runtime.InteropServices.ComTypes.FILETIME LastAccessTime;
            public System.Runtime.InteropServices.ComTypes.FILETIME LastWriteTime;
            public uint VolumeSerialNumber;
            public uint FileSizeHigh;
            public uint FileSizeLow;
            public uint NumberOfLinks;
            public uint FileIndexHigh;
            public uint FileIndexLow;
        }

        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        private static extern SafeFileHandle CreateFileW(
            string fileName,
            uint desiredAccess,
            uint shareMode,
            IntPtr securityAttributes,
            uint creationDisposition,
            uint flagsAndAttributes,
            IntPtr templateFile
        );

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool GetFileInformationByHandle(
            SafeFileHandle file,
            out BY_HANDLE_FILE_INFORMATION information
        );

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern uint GetFileType(SafeFileHandle file);

        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        private static extern bool MoveFileExW(string existingPath, string newPath, uint flags);

        private static void AssertSafeLockHandle(SafeFileHandle handle) {
            if (GetFileType(handle) != FILE_TYPE_DISK) {
                throw new InvalidOperationException(
                    "build publication lock path must be an unlinked regular file"
                );
            }

            BY_HANDLE_FILE_INFORMATION information;
            if (!GetFileInformationByHandle(handle, out information)) {
                throw new Win32Exception(Marshal.GetLastWin32Error());
            }
            if ((information.FileAttributes & (
                    FILE_ATTRIBUTE_DIRECTORY | FILE_ATTRIBUTE_REPARSE_POINT
                )) != 0 ||
                information.NumberOfLinks != 1) {
                throw new InvalidOperationException(
                    "build publication lock path must be an unlinked regular file"
                );
            }
        }

        public static FileStream OpenExclusiveLock(string path, byte[] owner) {
            SafeFileHandle handle = CreateFileW(
                path,
                GENERIC_READ | GENERIC_WRITE,
                0,
                IntPtr.Zero,
                OPEN_ALWAYS,
                FILE_ATTRIBUTE_NORMAL | FILE_FLAG_OPEN_REPARSE_POINT,
                IntPtr.Zero
            );
            if (handle.IsInvalid) {
                int error = Marshal.GetLastWin32Error();
                handle.Dispose();
                throw new Win32Exception(error);
            }

            try {
                AssertSafeLockHandle(handle);
                FileStream stream = new FileStream(handle, FileAccess.ReadWrite, 4096, false);
                handle = null;
                try {
                    AssertSafeLockHandle(stream.SafeFileHandle);
                    stream.SetLength(0);
                    stream.Write(owner, 0, owner.Length);
                    stream.Flush(true);
                    return stream;
                } catch {
                    stream.Dispose();
                    throw;
                }
            } finally {
                if (handle != null) handle.Dispose();
            }
        }

        public static bool AssertSafeTransactionDirectory(string path, bool allowMissing) {
            SafeFileHandle handle = CreateFileW(
                path,
                0,
                FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE,
                IntPtr.Zero,
                OPEN_EXISTING,
                FILE_FLAG_BACKUP_SEMANTICS | FILE_FLAG_OPEN_REPARSE_POINT,
                IntPtr.Zero
            );
            if (handle.IsInvalid) {
                int error = Marshal.GetLastWin32Error();
                handle.Dispose();
                if (allowMissing &&
                    (error == ERROR_FILE_NOT_FOUND || error == ERROR_PATH_NOT_FOUND)) {
                    return false;
                }
                throw new Win32Exception(error);
            }

            using (handle) {
                if (GetFileType(handle) != FILE_TYPE_DISK) {
                    throw new InvalidOperationException(
                        "build publication transaction path must be a local directory"
                    );
                }
                BY_HANDLE_FILE_INFORMATION information;
                if (!GetFileInformationByHandle(handle, out information)) {
                    throw new Win32Exception(Marshal.GetLastWin32Error());
                }
                if ((information.FileAttributes & FILE_ATTRIBUTE_DIRECTORY) == 0 ||
                    (information.FileAttributes & FILE_ATTRIBUTE_REPARSE_POINT) != 0) {
                    throw new InvalidOperationException(
                        "build publication transaction path must be a non-reparse directory"
                    );
                }
            }
            return true;
        }

        public static void MoveDurably(string source, string destination, bool replaceExisting) {
            uint flags = MOVEFILE_WRITE_THROUGH;
            if (replaceExisting) flags |= MOVEFILE_REPLACE_EXISTING;
            if (!MoveFileExW(source, destination, flags)) {
                throw new Win32Exception(Marshal.GetLastWin32Error());
            }
        }
    }
}
"@
}

function Move-BuildPublicationFileDurably {
    param(
        [Parameter(Mandatory = $true)][string]$From,
        [Parameter(Mandatory = $true)][string]$To,
        [switch]$ReplaceExisting
    )
    [ClipHub.BuildPublicationFiles]::MoveDurably($From, $To, [bool]$ReplaceExisting)
}

function Get-BuildPublicationJournalPath {
    param([Parameter(Mandatory = $true)][string]$BinDir)
    return Join-Path $BinDir ".build-publication.json"
}

function Enter-BuildPublicationLock {
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][string]$BinDir)

    $lockPath = Join-Path $BinDir ".build-publication.lock"
    try {
        $owner = [System.Text.UTF8Encoding]::new($false).GetBytes(
            "pid=$PID started=$([DateTimeOffset]::UtcNow.ToString('O'))"
        )
        $lock = [ClipHub.BuildPublicationFiles]::OpenExclusiveLock($lockPath, $owner)
        return $lock
    } catch {
        if ($null -ne $lock) {
            $lock.Dispose()
        }
        $lockError = $_.Exception
        while ($null -ne $lockError.InnerException) {
            $lockError = $lockError.InnerException
        }
        if ($lockError -is [System.IO.IOException] -or
            ($lockError -is [System.ComponentModel.Win32Exception] -and
             $lockError.NativeErrorCode -in @(32, 33))) {
            throw "another build publication is already active for $BinDir"
        }
        throw
    }
}

function Assert-BuildPublicationLock {
    param([Parameter(Mandatory = $true)][System.IO.FileStream]$PublicationLock)
    if (-not $PublicationLock.CanRead -or -not $PublicationLock.CanWrite) {
        throw "build publication lock is not active"
    }
}

function Assert-BuildPublicationChildDirectory {
    param(
        [Parameter(Mandatory = $true)][string]$BinDir,
        [Parameter(Mandatory = $true)][string]$Candidate,
        [Parameter(Mandatory = $true)][string]$LeafPattern
    )
    $binFull = [System.IO.Path]::GetFullPath($BinDir).TrimEnd(
        [System.IO.Path]::DirectorySeparatorChar,
        [System.IO.Path]::AltDirectorySeparatorChar
    )
    $candidateFull = [System.IO.Path]::GetFullPath($Candidate).TrimEnd(
        [System.IO.Path]::DirectorySeparatorChar,
        [System.IO.Path]::AltDirectorySeparatorChar
    )
    $parent = [System.IO.Path]::GetDirectoryName($candidateFull)
    $leaf = [System.IO.Path]::GetFileName($candidateFull)
    if (-not [string]::Equals($parent, $binFull, [System.StringComparison]::OrdinalIgnoreCase) -or
        $leaf -notmatch $LeafPattern) {
        throw "build publication recovery path is outside the expected bin transaction namespace"
    }
    return $candidateFull
}

function Assert-BuildPublicationTransactionDirectory {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [switch]$AllowMissing
    )
    try {
        return [ClipHub.BuildPublicationFiles]::AssertSafeTransactionDirectory(
            $Path,
            [bool]$AllowMissing
        )
    } catch {
        $inspectionError = $_.Exception
        while ($null -ne $inspectionError.InnerException) {
            $inspectionError = $inspectionError.InnerException
        }
        throw "unsafe build publication transaction directory: $Path. $($inspectionError.Message)"
    }
}

function Remove-BuildPublicationTransactionDirectory {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][scriptblock]$RemoveDirectory,
        [Parameter(Mandatory = $true)][string]$IncompleteMessage
    )
    $exists = Assert-BuildPublicationTransactionDirectory -Path $Path -AllowMissing
    if (-not $exists) {
        return
    }
    & $RemoveDirectory $Path
    if (Assert-BuildPublicationTransactionDirectory -Path $Path -AllowMissing) {
        throw $IncompleteMessage
    }
}

function Assert-BuildPublicationJournalItems {
    param(
        [Parameter(Mandatory = $true)][object]$Journal,
        [Parameter(Mandatory = $true)][string]$BinDir,
        [Parameter(Mandatory = $true)][string]$BackupDir
    )
    $itemsProperty = $Journal.PSObject.Properties["Items"]
    if ($null -eq $itemsProperty -or
        $null -eq $itemsProperty.Value -or
        $itemsProperty.Value -isnot [System.Array]) {
        throw "build publication journal items must be a non-empty collection"
    }
    $items = @($itemsProperty.Value)
    if ($items.Count -eq 0) {
        throw "build publication journal items must be a non-empty collection"
    }

    $uniqueNames = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::OrdinalIgnoreCase
    )
    foreach ($item in $items) {
        if ($null -eq $item -or $item -isnot [pscustomobject]) {
            throw "build publication journal contains an invalid artifact item"
        }
        $nameProperty = $item.PSObject.Properties["Name"]
        if ($null -eq $nameProperty -or
            $nameProperty.Value -isnot [string] -or
            [string]$nameProperty.Value -notmatch '^[A-Za-z0-9][A-Za-z0-9_-]*$') {
            throw "build publication journal contains an invalid artifact name"
        }
        $name = [string]$nameProperty.Value
        if (-not $uniqueNames.Add($name)) {
            throw "build publication journal contains duplicate artifact names"
        }

        $hadOriginalProperty = $item.PSObject.Properties["HadOriginal"]
        if ($null -eq $hadOriginalProperty -or
            $hadOriginalProperty.Value -isnot [bool]) {
            throw "build publication journal HadOriginal must be a boolean"
        }

        $targetProperty = $item.PSObject.Properties["Target"]
        $backupProperty = $item.PSObject.Properties["Backup"]
        if ($null -eq $targetProperty -or
            $targetProperty.Value -isnot [string] -or
            $null -eq $backupProperty -or
            $backupProperty.Value -isnot [string]) {
            throw "build publication journal contains invalid artifact paths"
        }
        try {
            $target = [System.IO.Path]::GetFullPath([string]$targetProperty.Value)
            $backup = [System.IO.Path]::GetFullPath([string]$backupProperty.Value)
        } catch {
            throw "build publication journal contains invalid artifact paths"
        }
        $expectedTarget = [System.IO.Path]::GetFullPath((Join-Path $BinDir "$name.exe"))
        $expectedBackup = [System.IO.Path]::GetFullPath((Join-Path $BackupDir "$name.exe"))
        if (-not [string]::Equals($target, $expectedTarget, [StringComparison]::OrdinalIgnoreCase) -or
            -not [string]::Equals($backup, $expectedBackup, [StringComparison]::OrdinalIgnoreCase)) {
            throw "build publication journal contains invalid artifact paths"
        }

        if ([int]$Journal.SchemaVersion -eq 2) {
            $phaseProperty = $item.PSObject.Properties["Phase"]
            if ($null -eq $phaseProperty -or $phaseProperty.Value -isnot [string]) {
                throw "build publication journal contains an invalid artifact phase"
            }
            $itemPhase = [string]$phaseProperty.Value
            if ($itemPhase -ne "pending" -and
                $itemPhase -ne "backup_created" -and
                $itemPhase -ne "published" -and
                $itemPhase -ne "restoring" -and
                $itemPhase -ne "restored") {
                throw "build publication journal contains an invalid artifact phase"
            }
        }
    }
    return $items
}

function Write-BuildPublicationJournal {
    param(
        [Parameter(Mandatory = $true)][string]$JournalPath,
        [Parameter(Mandatory = $true)][object]$Document
    )
    $temporary = "$JournalPath.tmp-$([guid]::NewGuid().ToString('N'))"
    $json = $Document | ConvertTo-Json -Depth 5
    $bytes = [System.Text.UTF8Encoding]::new($false).GetBytes($json)
    try {
        $stream = [System.IO.FileStream]::new(
            $temporary,
            [System.IO.FileMode]::CreateNew,
            [System.IO.FileAccess]::Write,
            [System.IO.FileShare]::None
        )
        try {
            $stream.Write($bytes, 0, $bytes.Length)
            $stream.Flush($true)
        } finally {
            $stream.Dispose()
        }
        Move-BuildPublicationFileDurably `
            -From $temporary `
            -To $JournalPath `
            -ReplaceExisting:(Test-Path -LiteralPath $JournalPath -PathType Leaf)
    } finally {
        if (Test-Path -LiteralPath $temporary) {
            Remove-Item -LiteralPath $temporary -Force
        }
    }
}

function Test-BuildPublicationCommittedJournal {
    param([Parameter(Mandatory = $true)][string]$JournalPath)
    try {
        $journal = Get-Content -LiteralPath $JournalPath -Raw | ConvertFrom-Json
        return [int]$journal.SchemaVersion -eq 2 -and [string]$journal.Phase -eq "committed"
    } catch {
        return $false
    }
}

function Set-BuildPublicationItemPhase {
    param(
        [Parameter(Mandatory = $true)][object]$Document,
        [Parameter(Mandatory = $true)][object]$Item,
        [Parameter(Mandatory = $true)][string]$Phase,
        [Parameter(Mandatory = $true)][string]$JournalPath
    )
    if ([int]$Document.SchemaVersion -ne 2) {
        return
    }
    $Item.Phase = $Phase
    Write-BuildPublicationJournal -JournalPath $JournalPath -Document $Document
}

function Remove-BuildPublicationCandidate {
    param(
        [Parameter(Mandatory = $true)][string]$Target,
        [Parameter(Mandatory = $true)][scriptblock]$RemoveFile
    )
    if (-not (Test-Path -LiteralPath $Target)) {
        return
    }
    & $RemoveFile $Target
    if (Test-Path -LiteralPath $Target) {
        throw "candidate removal did not complete"
    }
}

function Restore-BuildPublicationOriginal {
    param(
        [Parameter(Mandatory = $true)][object]$Document,
        [Parameter(Mandatory = $true)][object]$Item,
        [Parameter(Mandatory = $true)][string]$JournalPath,
        [Parameter(Mandatory = $true)][scriptblock]$MoveFile,
        [Parameter(Mandatory = $true)][scriptblock]$RemoveFile
    )
    $targetExists = Test-Path -LiteralPath $Item.Target
    $backupExists = Test-Path -LiteralPath $Item.Backup -PathType Leaf
    $itemPhase = if ([int]$Document.SchemaVersion -eq 2) {
        [string]$Item.Phase
    } else {
        "unknown"
    }

    if ($backupExists) {
        if ([int]$Document.SchemaVersion -ne 2) {
            # Legacy journals have no durable per-item phase. Preserve their
            # backup until whole-transaction cleanup so an interruption after
            # the copy remains retryable.
            if ($targetExists) {
                Remove-BuildPublicationCandidate -Target $Item.Target -RemoveFile $RemoveFile
            }
            [System.IO.File]::Copy($Item.Backup, $Item.Target)
            if (-not (Test-Path -LiteralPath $Item.Backup -PathType Leaf) -or
                -not (Test-Path -LiteralPath $Item.Target -PathType Leaf)) {
                throw "legacy original restore copy did not complete"
            }
            return
        }
        Set-BuildPublicationItemPhase -Document $Document -Item $Item -Phase "restoring" -JournalPath $JournalPath
        if ($targetExists) {
            Remove-BuildPublicationCandidate -Target $Item.Target -RemoveFile $RemoveFile
        }
        try {
            & $MoveFile $Item.Backup $Item.Target
        } catch {
            if ((Test-Path -LiteralPath $Item.Backup -PathType Leaf) -or
                -not (Test-Path -LiteralPath $Item.Target -PathType Leaf)) {
                throw
            }
        }
        if ((Test-Path -LiteralPath $Item.Backup -PathType Leaf) -or
            -not (Test-Path -LiteralPath $Item.Target -PathType Leaf)) {
            throw "original restore move did not complete"
        }
        Set-BuildPublicationItemPhase -Document $Document -Item $Item -Phase "restored" -JournalPath $JournalPath
        return
    }

    if (($itemPhase -eq "restoring" -or $itemPhase -eq "restored") -and
        (Test-Path -LiteralPath $Item.Target -PathType Leaf)) {
        Set-BuildPublicationItemPhase -Document $Document -Item $Item -Phase "restored" -JournalPath $JournalPath
        return
    }

    if (-not $targetExists) {
        throw "original and backup are both missing"
    }
    throw "original backup is missing after phase '$itemPhase'; target was retained"
}

function Recover-BuildPublication {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$BinDir,
        [System.IO.FileStream]$PublicationLock,
        [scriptblock]$MoveFile = {
            param([string]$From, [string]$To)
            Move-BuildPublicationFileDurably -From $From -To $To
        },
        [scriptblock]$RemoveFile = {
            param([string]$Path)
            Remove-Item -LiteralPath $Path -Force -ErrorAction Stop
        },
        [scriptblock]$RemoveDirectory = {
            param([string]$Path)
            Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction Stop
        }
    )

    $ownedLock = $null
    if ($null -eq $PublicationLock) {
        $ownedLock = Enter-BuildPublicationLock -BinDir $BinDir
        $PublicationLock = $ownedLock
    }
    Assert-BuildPublicationLock -PublicationLock $PublicationLock

    try {
        $journalPath = Get-BuildPublicationJournalPath -BinDir $BinDir
        if (-not (Test-Path -LiteralPath $journalPath -PathType Leaf)) {
            return
        }
        try {
            $journal = Get-Content -LiteralPath $journalPath -Raw | ConvertFrom-Json
        } catch {
            throw "build publication journal is unreadable; refusing destructive recovery"
        }
        $phase = "publishing"
        switch ([int]$journal.SchemaVersion) {
            1 {
                # Schema 1 predates the durable committed phase. Its journal
                # always means publication may have stopped mid-move.
            }
            2 {
                $phase = [string]$journal.Phase
                if ($phase -ne "publishing" -and $phase -ne "committed") {
                    throw "build publication journal contains an invalid phase"
                }
            }
            default {
                throw "unsupported build publication journal schema"
            }
        }
        $backupDir = Assert-BuildPublicationChildDirectory -BinDir $BinDir -Candidate $journal.BackupDirectory -LeafPattern '^\.backup-[A-Za-z0-9]+$'
        $stagingDir = Assert-BuildPublicationChildDirectory -BinDir $BinDir -Candidate $journal.StagingDirectory -LeafPattern '^\.build-[A-Za-z0-9]+$'
        [void](Assert-BuildPublicationTransactionDirectory -Path $backupDir -AllowMissing)
        [void](Assert-BuildPublicationTransactionDirectory -Path $stagingDir -AllowMissing)
        $items = @(Assert-BuildPublicationJournalItems -Journal $journal -BinDir $BinDir -BackupDir $backupDir)

        if ($phase -eq "committed") {
            $invalidTargets = [System.Collections.Generic.List[string]]::new()
            foreach ($item in $items) {
                $target = Join-Path $BinDir "$($item.Name).exe"
                try {
                    if ([string]$item.Phase -ne "published") {
                        throw "artifact was not durably published"
                    }
                    $artifact = Get-Item -LiteralPath $target
                    if ($artifact.PSIsContainer -or $artifact.Length -le 0) {
                        throw "target is missing or empty"
                    }
                } catch {
                    $invalidTargets.Add(("{0}: {1}" -f $target, $_.Exception.Message))
                }
            }
            if ($invalidTargets.Count -gt 0) {
                throw ("committed build publication is incomplete; recovery artifacts were retained: {0}" -f ($invalidTargets -join "; "))
            }
            foreach ($directory in @($backupDir, $stagingDir)) {
                Remove-BuildPublicationTransactionDirectory `
                    -Path $directory `
                    -RemoveDirectory $RemoveDirectory `
                    -IncompleteMessage "committed build publication directory cleanup did not complete: $directory"
            }
            & $RemoveFile $journalPath
            if (Test-Path -LiteralPath $journalPath -PathType Leaf) {
                throw "committed build publication journal cleanup did not complete"
            }
            return
        }

        $recoveryErrors = [System.Collections.Generic.List[string]]::new()
        for ($i = $items.Count - 1; $i -ge 0; $i--) {
            $item = $items[$i]
            $name = [string]$item.Name
            $target = Join-Path $BinDir "$name.exe"
            $backup = Join-Path $backupDir "$name.exe"
            try {
                # Ignore serialized paths for execution, but keep the validated
                # transaction paths on the in-memory item used by shared
                # idempotent restore helpers.
                $item.Target = $target
                $item.Backup = $backup
                $itemPhase = if ([int]$journal.SchemaVersion -eq 2) {
                    [string]$item.Phase
                } else {
                    "unknown"
                }
                if ([bool]$item.HadOriginal) {
                    if ($itemPhase -eq "pending" -and
                        -not (Test-Path -LiteralPath $backup -PathType Leaf) -and
                        (Test-Path -LiteralPath $target -PathType Leaf)) {
                        # Staged publication cannot begin before a durable
                        # backup_created phase, so this is still the original.
                        Set-BuildPublicationItemPhase -Document $journal -Item $item -Phase "restored" -JournalPath $journalPath
                    } else {
                        Restore-BuildPublicationOriginal -Document $journal -Item $item -JournalPath $journalPath -MoveFile $MoveFile -RemoveFile $RemoveFile
                    }
                } else {
                    Set-BuildPublicationItemPhase -Document $journal -Item $item -Phase "restoring" -JournalPath $journalPath
                    Remove-BuildPublicationCandidate -Target $target -RemoveFile $RemoveFile
                    Set-BuildPublicationItemPhase -Document $journal -Item $item -Phase "restored" -JournalPath $journalPath
                }
            } catch {
                $recoveryErrors.Add(("{0}: {1}" -f $target, $_.Exception.Message))
            }
        }
        if ($recoveryErrors.Count -gt 0) {
            throw ("build publication recovery was incomplete; journal and recovery artifacts were retained: {0}" -f ($recoveryErrors -join "; "))
        }
        if ([int]$journal.SchemaVersion -eq 1) {
            # Schema 1 cannot represent that every original has been restored.
            # Upgrade the journal atomically before deleting its backups so an
            # interruption during cleanup resumes from restart-safe item phases.
            $journal.SchemaVersion = 2
            $journal | Add-Member -NotePropertyName Phase -NotePropertyValue "publishing" -Force
            foreach ($item in $items) {
                $item | Add-Member -NotePropertyName Phase -NotePropertyValue "restored" -Force
            }
            Write-BuildPublicationJournal -JournalPath $journalPath -Document $journal
        }
        foreach ($directory in @($backupDir, $stagingDir)) {
            Remove-BuildPublicationTransactionDirectory `
                -Path $directory `
                -RemoveDirectory $RemoveDirectory `
                -IncompleteMessage "build publication recovery directory cleanup did not complete: $directory"
        }
        Remove-Item -LiteralPath $journalPath -Force
        Remove-Item -LiteralPath "$journalPath.incomplete" -Force -ErrorAction SilentlyContinue
    } finally {
        if ($null -ne $ownedLock) {
            $ownedLock.Dispose()
        }
    }
}

function Publish-BuildArtifacts {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string[]]$Names,
        [Parameter(Mandatory = $true)]
        [string]$BinDir,
        [Parameter(Mandatory = $true)]
        [string]$StagingDir,
        [Parameter(Mandatory = $true)]
        [string]$BackupDir,
        [scriptblock]$MoveFile = {
            param([string]$From, [string]$To)
            Move-BuildPublicationFileDurably -From $From -To $To
        },
        [scriptblock]$RemoveFile = {
            param([string]$Path)
            Remove-Item -LiteralPath $Path -Force -ErrorAction Stop
        },
        [scriptblock]$RemoveDirectory = {
            param([string]$Path)
            Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction Stop
        },
        [System.IO.FileStream]$PublicationLock
    )

    # Validate the complete logical key set before recovery, journaling, or any
    # artifact move. Windows paths are case-insensitive, so "zv" and "ZV"
    # would otherwise target the same executable twice inside one transaction.
    $uniqueNames = [System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::OrdinalIgnoreCase
    )
    foreach ($name in $Names) {
        if ($name -notmatch '^[A-Za-z0-9][A-Za-z0-9_-]*$') {
            throw "invalid build artifact name"
        }
        if (-not $uniqueNames.Add($name)) {
            throw "duplicate build artifact name: $name"
        }
    }

    $resolvedStaging = Assert-BuildPublicationChildDirectory -BinDir $BinDir -Candidate $StagingDir -LeafPattern '^\.build-[A-Za-z0-9]+$'
    $resolvedBackup = Assert-BuildPublicationChildDirectory -BinDir $BinDir -Candidate $BackupDir -LeafPattern '^\.backup-[A-Za-z0-9]+$'
    [void](Assert-BuildPublicationTransactionDirectory -Path $resolvedStaging)
    if (Assert-BuildPublicationTransactionDirectory -Path $resolvedBackup -AllowMissing) {
        throw "build publication backup directory already exists: $resolvedBackup"
    }

    $ownedLock = $null
    if ($null -eq $PublicationLock) {
        $ownedLock = Enter-BuildPublicationLock -BinDir $BinDir
        $PublicationLock = $ownedLock
    }
    Assert-BuildPublicationLock -PublicationLock $PublicationLock

    try {
        Recover-BuildPublication -BinDir $BinDir -PublicationLock $PublicationLock -MoveFile $MoveFile -RemoveFile $RemoveFile -RemoveDirectory $RemoveDirectory

        [void](Assert-BuildPublicationTransactionDirectory -Path $resolvedStaging)
        if (Assert-BuildPublicationTransactionDirectory -Path $resolvedBackup -AllowMissing) {
            throw "build publication backup directory already exists: $resolvedBackup"
        }

        $targetPresence = @{}
        foreach ($name in $Names) {
            $target = Join-Path $BinDir "$name.exe"
            $hadOriginal = $false
            try {
                $attributes = [System.IO.File]::GetAttributes($target)
                $hadOriginal = $true
            } catch [System.IO.FileNotFoundException] {
                # A missing target is a valid first publication.
            } catch [System.IO.DirectoryNotFoundException] {
                # A missing target is a valid first publication.
            } catch {
                throw "could not inspect build artifact target before publication: $target"
            }
            if ($hadOriginal -and (
                ($attributes -band [System.IO.FileAttributes]::Directory) -ne 0 -or
                ($attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0
            )) {
                throw "build artifact target is not a regular file: $target"
            }
            $targetPresence[$name] = $hadOriginal
        }

        $journalPath = Get-BuildPublicationJournalPath -BinDir $BinDir
        $transactionItems = [System.Collections.Generic.List[object]]::new()
        foreach ($name in $Names) {
            $transactionItems.Add([pscustomobject]@{
                Name = $name
                Target = Join-Path $BinDir "$name.exe"
                Backup = Join-Path $resolvedBackup "$name.exe"
                HadOriginal = [bool]$targetPresence[$name]
                Phase = "pending"
            })
        }
        $journalDocument = [pscustomobject]@{
            SchemaVersion = 2
            Phase = "publishing"
            BackupDirectory = $resolvedBackup
            StagingDirectory = $resolvedStaging
            Items = @($transactionItems)
        }
        Write-BuildPublicationJournal -JournalPath $journalPath -Document $journalDocument

        $backupCanBeRemoved = $false
        $committed = $false
        $attempted = [System.Collections.Generic.List[object]]::new()

        try {
            [void](New-Item -ItemType Directory -Path $resolvedBackup)
            [void](Assert-BuildPublicationTransactionDirectory -Path $resolvedStaging)
            [void](Assert-BuildPublicationTransactionDirectory -Path $resolvedBackup)
            foreach ($item in $transactionItems) {
                # Register before the first move. Even an injected/filesystem move
                # that completes and then reports failure remains recoverable.
                $attempted.Add($item)
                $staged = Join-Path $resolvedStaging "$($item.Name).exe"
                if ($item.HadOriginal) {
                    & $MoveFile $item.Target $item.Backup
                }
                $item.Phase = "backup_created"
                Write-BuildPublicationJournal -JournalPath $journalPath -Document $journalDocument
                & $MoveFile $staged $item.Target
                $item.Phase = "published"
                Write-BuildPublicationJournal -JournalPath $journalPath -Document $journalDocument
            }
            foreach ($item in $transactionItems) {
                $artifact = Get-Item -LiteralPath $item.Target
                if ($artifact.PSIsContainer -or $artifact.Length -le 0) {
                    throw "published build artifact is missing or empty: $($item.Name)"
                }
            }
            # Once this atomically replaced journal is durable, recovery must
            # preserve the complete new generation and only finish cleanup.
            $journalDocument.Phase = "committed"
            Write-BuildPublicationJournal -JournalPath $journalPath -Document $journalDocument
            $committed = $true
            foreach ($directory in @($resolvedBackup, $resolvedStaging)) {
                Remove-BuildPublicationTransactionDirectory `
                    -Path $directory `
                    -RemoveDirectory $RemoveDirectory `
                    -IncompleteMessage "committed build publication directory cleanup did not complete: $directory"
            }
            & $RemoveFile $journalPath
            if (Test-Path -LiteralPath $journalPath -PathType Leaf) {
                throw "committed build publication journal cleanup did not complete"
            }
            Remove-Item -LiteralPath "$journalPath.incomplete" -Force -ErrorAction SilentlyContinue
        } catch {
            $publicationError = $_
            if (-not $committed) {
                $committed = Test-BuildPublicationCommittedJournal -JournalPath $journalPath
            }
            if ($committed) {
                throw ("Build publication committed, but cleanup did not finish. The new artifact set was retained. Cleanup error: {0}" -f $publicationError.Exception.Message)
            }
            $rollbackErrors = [System.Collections.Generic.List[string]]::new()
            for ($i = $attempted.Count - 1; $i -ge 0; $i--) {
                $item = $attempted[$i]
                try {
                    $backupExists = Test-Path -LiteralPath $item.Backup -PathType Leaf
                    if ($item.HadOriginal -and -not $backupExists) {
                        $targetExists = Test-Path -LiteralPath $item.Target -PathType Leaf
                        if ([string]$item.Phase -eq "pending" -and $targetExists) {
                            # The staged move is ordered after a durable
                            # backup_created phase, so pending + target proves
                            # this is still the original generation.
                            Set-BuildPublicationItemPhase -Document $journalDocument -Item $item -Phase "restored" -JournalPath $journalPath
                            continue
                        }
                    }
                    if ($item.HadOriginal) {
                        Restore-BuildPublicationOriginal -Document $journalDocument -Item $item -JournalPath $journalPath -MoveFile $MoveFile -RemoveFile $RemoveFile
                    } else {
                        Set-BuildPublicationItemPhase -Document $journalDocument -Item $item -Phase "restoring" -JournalPath $journalPath
                        Remove-BuildPublicationCandidate -Target $item.Target -RemoveFile $RemoveFile
                        Set-BuildPublicationItemPhase -Document $journalDocument -Item $item -Phase "restored" -JournalPath $journalPath
                    }
                } catch {
                    $rollbackErrors.Add(("{0}: {1}" -f $item.Target, $_.Exception.Message))
                }
            }
            if ($rollbackErrors.Count -eq 0) {
                Remove-Item -LiteralPath $journalPath -Force
                Remove-Item -LiteralPath "$journalPath.incomplete" -Force -ErrorAction SilentlyContinue
                $backupCanBeRemoved = $true
                throw $publicationError
            }
            [System.IO.File]::WriteAllText(
                "$journalPath.incomplete",
                "Manual recovery required: one or more original build artifacts could not be restored.",
                [System.Text.UTF8Encoding]::new($false)
            )
            throw ("Build publication failed and rollback was incomplete. Recovery artifacts and journal were retained at {0}. Publication error: {1}. Rollback errors: {2}" -f $resolvedBackup, $publicationError.Exception.Message, ($rollbackErrors -join "; "))
        } finally {
            if ($backupCanBeRemoved) {
                Remove-BuildPublicationTransactionDirectory `
                    -Path $resolvedBackup `
                    -RemoveDirectory $RemoveDirectory `
                    -IncompleteMessage "build publication backup cleanup did not complete: $resolvedBackup"
            }
        }
    } finally {
        if ($null -ne $ownedLock) {
            $ownedLock.Dispose()
        }
    }
}
