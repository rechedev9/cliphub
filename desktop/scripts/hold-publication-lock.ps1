param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("acquire", "release")]
    [string]$Mode,
    [Parameter(Mandatory = $true)]
    [string]$LockPath,
    [Parameter(Mandatory = $true)]
    [int]$OwnerPID,
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[0-9a-f]{32}$')]
    [string]$Token
)

$ErrorActionPreference = "Stop"
$lock = $null

try {
    Add-Type -TypeDefinition @'
using System;
using System.ComponentModel;
using System.IO;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

namespace ClipHub {
    public sealed class PublicationLockContendedException : IOException {
        public PublicationLockContendedException()
            : base("another distribution build is already running") {}
    }

    public static class PublicationLockFile {
        private const uint GenericRead = 0x80000000;
        private const uint GenericWrite = 0x40000000;
        private const uint OpenAlways = 4;
        private const uint FileAttributeDirectory = 0x00000010;
        private const uint FileAttributeNormal = 0x00000080;
        private const uint FileAttributeReparsePoint = 0x00000400;
        private const uint FileFlagOpenReparsePoint = 0x00200000;
        private const uint FileTypeDisk = 0x0001;
        private const int ErrorSharingViolation = 32;
        private const int ErrorLockViolation = 33;

        [StructLayout(LayoutKind.Sequential)]
        private struct ByHandleFileInformation {
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
        [return: MarshalAs(UnmanagedType.Bool)]
        private static extern bool GetFileInformationByHandle(
            SafeFileHandle file,
            out ByHandleFileInformation information
        );

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern uint GetFileType(SafeFileHandle file);

        public static SafeFileHandle OpenValidated(string path) {
            SafeFileHandle handle = CreateFileW(
                path,
                GenericRead | GenericWrite,
                0,
                IntPtr.Zero,
                OpenAlways,
                FileAttributeNormal | FileFlagOpenReparsePoint,
                IntPtr.Zero
            );
            if (handle.IsInvalid) {
                int error = Marshal.GetLastWin32Error();
                handle.Dispose();
                if (error == ErrorSharingViolation || error == ErrorLockViolation) {
                    throw new PublicationLockContendedException();
                }
                throw new Win32Exception(error, "could not open publication lock");
            }

            try {
                ByHandleFileInformation information;
                if (!GetFileInformationByHandle(handle, out information)) {
                    throw new Win32Exception(
                        Marshal.GetLastWin32Error(),
                        "could not inspect publication lock"
                    );
                }
                if (GetFileType(handle) != FileTypeDisk) {
                    throw new InvalidDataException(
                        "publication lock path is not a regular disk file"
                    );
                }
                if ((information.FileAttributes & FileAttributeReparsePoint) != 0) {
                    throw new InvalidDataException(
                        "publication lock path must not be a reparse point"
                    );
                }
                if ((information.FileAttributes & FileAttributeDirectory) != 0) {
                    throw new InvalidDataException(
                        "publication lock path is not a regular file"
                    );
                }
                if (information.NumberOfLinks != 1) {
                    throw new InvalidDataException(
                        "publication lock path must have exactly one hard link"
                    );
                }
                return handle;
            } catch {
                handle.Dispose();
                throw;
            }
        }
    }
}
'@ -ErrorAction Stop
} catch {
    [Console]::Error.WriteLine("publication lock native validation is unavailable: $($_.Exception.Message)")
    exit 1
}

function Test-LockContention {
    param([System.Management.Automation.ErrorRecord]$ErrorRecord)
    $exception = $ErrorRecord.Exception
    while ($null -ne $exception) {
        if ($exception -is [ClipHub.PublicationLockContendedException]) {
            return $true
        }
        $exception = $exception.InnerException
    }
    return $false
}

function Get-ProcessStartIdentity {
    param([int]$ProcessID)
    try {
        $process = Get-Process -Id $ProcessID -ErrorAction Stop
    } catch {
        return ""
    }
    try {
        return $process.StartTime.ToUniversalTime().Ticks.ToString(
            [Globalization.CultureInfo]::InvariantCulture
        )
    } catch {
        throw "publication lock owner identity cannot be verified"
    }
}

function Read-LockDocument {
    param([System.IO.FileStream]$Stream)
    if ($Stream.Length -eq 0) {
        return $null
    }
    if ($Stream.Length -gt 16384) {
        throw "publication lock state is invalid"
    }
    $bytes = New-Object byte[] ([int]$Stream.Length)
    $Stream.Position = 0
    $read = $Stream.Read($bytes, 0, $bytes.Length)
    if ($read -ne $bytes.Length) {
        throw "publication lock state is unreadable"
    }
    try {
        return ([Text.Encoding]::UTF8.GetString($bytes) | ConvertFrom-Json)
    } catch {
        throw "publication lock state is unreadable"
    }
}

function Test-LockDocument {
    param([object]$Document)
    if ($null -eq $Document) {
        return $false
    }
    return [int]$Document.schema_version -eq 1 -and
        ([string]$Document.state -eq "held" -or [string]$Document.state -eq "released") -and
        [int]$Document.owner_pid -gt 0 -and
        [string]$Document.owner_started_ticks -match '^[0-9]+$' -and
        [string]$Document.token -match '^[0-9a-f]{32}$'
}

function Test-LiveOwner {
    param([object]$Document)
    if ([string]$Document.state -ne "held") {
        return $false
    }
    $currentIdentity = Get-ProcessStartIdentity -ProcessID ([int]$Document.owner_pid)
    return -not [string]::IsNullOrWhiteSpace($currentIdentity) -and
        $currentIdentity -eq [string]$Document.owner_started_ticks
}

function Write-LockDocument {
    param(
        [System.IO.FileStream]$Stream,
        [object]$Document
    )
    $json = $Document | ConvertTo-Json -Compress
    $bytes = [Text.UTF8Encoding]::new($false).GetBytes($json)
    $Stream.Position = 0
    $Stream.SetLength(0)
    $Stream.Write($bytes, 0, $bytes.Length)
    $Stream.Flush($true)
}

try {
    for ($attempt = 0; $attempt -lt 20; $attempt++) {
        try {
            # CreateFileW opens the final component without following a reparse
            # point. The native helper validates this exact exclusive handle as
            # a regular, single-link disk file before FileStream can read or
            # truncate it.
            $validatedHandle = [ClipHub.PublicationLockFile]::OpenValidated($LockPath)
            try {
                $lock = [System.IO.FileStream]::new(
                    $validatedHandle,
                    [System.IO.FileAccess]::ReadWrite
                )
                $validatedHandle = $null
            } finally {
                if ($null -ne $validatedHandle) {
                    $validatedHandle.Dispose()
                }
            }
            break
        } catch {
            if (-not (Test-LockContention -ErrorRecord $_)) {
                throw
            }
            if ($attempt -eq 19) {
                [Console]::Error.WriteLine("another distribution build is already running")
                exit 2
            }
            Start-Sleep -Milliseconds 25
        }
    }

    $document = Read-LockDocument -Stream $lock
    if ($null -ne $document -and -not (Test-LockDocument -Document $document)) {
        throw "publication lock state is invalid"
    }

    if ($Mode -eq "release") {
        if ($null -ne $document -and
            [string]$document.state -eq "held" -and
            ([int]$document.owner_pid -ne $OwnerPID -or [string]$document.token -ne $Token)) {
            throw "publication lock ownership changed; refusing release"
        }
        $ownerIdentity = if ($null -ne $document) {
            [string]$document.owner_started_ticks
        } else {
            Get-ProcessStartIdentity -ProcessID $OwnerPID
        }
        Write-LockDocument -Stream $lock -Document ([pscustomobject]@{
            schema_version = 1
            state = "released"
            owner_pid = $OwnerPID
            owner_started_ticks = $ownerIdentity
            token = $Token
        })
        [Console]::Out.WriteLine("RELEASED")
        [Console]::Out.Flush()
        exit 0
    }

    if ($null -ne $document -and
        [string]$document.state -eq "held" -and
        (Test-LiveOwner -Document $document)) {
        [Console]::Error.WriteLine("another distribution build is already running")
        exit 2
    }

    $ownerIdentity = Get-ProcessStartIdentity -ProcessID $OwnerPID
    if ([string]::IsNullOrWhiteSpace($ownerIdentity)) {
        throw "publication lock owner identity is unavailable"
    }
    $document = [pscustomobject]@{
        schema_version = 1
        state = "held"
        owner_pid = $OwnerPID
        owner_started_ticks = $ownerIdentity
        token = $Token
    }
    Write-LockDocument -Stream $lock -Document $document
    [Console]::Out.WriteLine("LOCKED")
    [Console]::Out.Flush()

    # RELEASE is an explicit owner action. EOF means the Node owner died, so
    # the held fence remains for the next contender to reclaim by PID+identity.
    $command = [Console]::In.ReadLine()
    if ($command -eq "RELEASE") {
        $document.state = "released"
        Write-LockDocument -Stream $lock -Document $document
        [Console]::Out.WriteLine("RELEASED")
        [Console]::Out.Flush()
    }
} catch {
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 1
} finally {
    if ($null -ne $lock) {
        $lock.Dispose()
    }
}
