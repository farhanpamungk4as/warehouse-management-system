import type { ReactNode } from "react";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "./select";
export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}
interface Props {
  id?: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: SelectOption[];
  placeholder?: string;
  disabled?: boolean;
  invalid?: boolean;
  describedBy?: string;
  className?: string;
  icon?: ReactNode;
  groupLabel?: string;
}
// Prefix actual values so the empty "all" option never collides with a real
// category. Radix reserves an empty item value for its placeholder.
const encode = (value: string) => `option:${value}`;
export function SelectField({
  id,
  label,
  value,
  onChange,
  options,
  placeholder,
  disabled,
  invalid,
  describedBy,
  className = "",
  icon,
  groupLabel,
}: Props) {
  const hasEmpty = options.some((option) => option.value === "");
  return (
    <Select
      value={value === "" && !hasEmpty ? "" : encode(value)}
      onValueChange={(value) => onChange(value.slice(7))}
      disabled={disabled}
    >
      <SelectTrigger
        id={id}
        aria-label={label}
        aria-invalid={invalid || undefined}
        aria-describedby={describedBy}
        className={`select-field ${className}`}
      >
        {icon}
        <SelectValue placeholder={placeholder ?? label} />
      </SelectTrigger>
      <SelectContent
        position="popper"
        align="start"
        sideOffset={5}
        collisionPadding={12}
        className="select-field-content"
      >
        <SelectGroup>
          {groupLabel && <SelectLabel>{groupLabel}</SelectLabel>}
          {options.map((option) => (
            <SelectItem
              key={option.value}
              value={encode(option.value)}
              disabled={option.disabled}
            >
              {option.label}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  );
}
