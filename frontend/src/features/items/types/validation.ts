import type { ItemErrors, ItemField, ItemWrite } from "./item";

// Unicode White_Space matches Go unicode.IsSpace (including NEL U+0085).
export const isUnicodeWhitespaceOnly = (value: string) =>
  /^[\u0009-\u000d\u0020\u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]*$/.test(
    value,
  );

export const fieldDefinitions: {
  name: ItemField;
  label: string;
  max: number;
  hint: string;
  placeholder: string;
}[] = [
  {
    name: "sku",
    label: "SKU",
    max: 64,
    hint: "A unique identifier. SKU is case-sensitive.",
    placeholder: "e.g. WH-001",
  },
  {
    name: "name",
    label: "Item name",
    max: 200,
    hint: "Use a clear, recognizable item name.",
    placeholder: "e.g. Storage Box",
  },
  {
    name: "category",
    label: "Category",
    max: 64,
    hint: "Group related items with a category.",
    placeholder: "e.g. Packaging",
  },
  {
    name: "unit",
    label: "Unit",
    max: 32,
    hint: "The unit used to count this item.",
    placeholder: "e.g. pcs, box, kg",
  },
];
export function validateItem(input: ItemWrite): {
  values: ItemWrite;
  errors: ItemErrors;
} {
  const values = { ...input };
  const errors: ItemErrors = {};
  for (const field of fieldDefinitions) {
    const value = input[field.name].replace(/^ +| +$/g, "");
    values[field.name] = value;
    if (isUnicodeWhitespaceOnly(value))
      errors[field.name] = `${field.label} is required.`;
    else if (/[\u0000-\u001f\u007f]/.test(value))
      errors[field.name] = "Remove tabs, line breaks, and control characters.";
    else if (
      /[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/.test(
        value,
      )
    )
      errors[field.name] = "Enter valid Unicode text.";
    else if ([...value].length > field.max)
      errors[field.name] = `Use ${field.max} characters or fewer.`;
  }
  return { values, errors };
}
