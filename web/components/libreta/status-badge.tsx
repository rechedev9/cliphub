import type { ReactNode } from 'react';
import { LIBRETA_STATUSES, isLibretaStatus, type LibretaStatus } from '@/lib/api/libreta';
import { STATUS_LABEL, STATUS_TONE } from '@/lib/libreta/status';
import { Badge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

export function StatusBadge({ status }: { status: LibretaStatus }): ReactNode {
  return <Badge variant={STATUS_TONE[status]}>{STATUS_LABEL[status]}</Badge>;
}

export function StatusField({ value, onChange }: { value: LibretaStatus; onChange: (status: LibretaStatus) => void }): ReactNode {
  return (
    <Field label="Estado">
      {(control) => (
        <Select value={value} onValueChange={(next) => isLibretaStatus(next) && onChange(next)}>
          <SelectTrigger {...control} className="h-9 w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {LIBRETA_STATUSES.map((status) => (
              <SelectItem key={status} value={status}>
                {STATUS_LABEL[status]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </Field>
  );
}

export const STATUS_FILTER_OPTIONS = LIBRETA_STATUSES.map((value) => ({ value, label: STATUS_LABEL[value] }));
