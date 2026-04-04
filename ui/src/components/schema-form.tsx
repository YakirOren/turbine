import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { CodeMirrorEditor } from "@/components/codemirror";

export interface SchemaField {
  name: string;
  type: "string" | "number" | "boolean" | "select" | "multiselect" | "textarea" | "json";
  label?: string;
  required?: boolean;
  default?: unknown;
  options?: string[];
  placeholder?: string;
  description?: string;
}

export interface InputSchema {
  fields: SchemaField[];
}

export function SchemaFormField({
  field,
  value,
  onChange,
  id,
  error,
}: {
  field: SchemaField;
  value: unknown;
  onChange: (value: unknown) => void;
  id: string;
  error?: { message?: string };
}) {
  const fieldId = `${id}-field-${field.name}`;
  const label = field.label ?? field.name;

  if (field.type === "boolean") {
    return (
      <Field orientation="horizontal">
        <div>
          <FieldLabel htmlFor={fieldId}>{label}</FieldLabel>
          {field.description && (
            <FieldDescription>{field.description}</FieldDescription>
          )}
        </div>
        <Switch
          id={fieldId}
          checked={!!value}
          onCheckedChange={(checked) => onChange(checked)}
        />
      </Field>
    );
  }

  if (field.type === "select" && field.options) {
    return (
      <Field data-invalid={!!error}>
        <FieldLabel htmlFor={fieldId}>
          {label}
          {field.required && <span className="text-destructive">*</span>}
        </FieldLabel>
        {field.description && (
          <FieldDescription>{field.description}</FieldDescription>
        )}
        <Select value={String(value ?? "")} onValueChange={(v) => onChange(v)}>
          <SelectTrigger id={fieldId} aria-invalid={!!error}>
            <SelectValue placeholder={`Select ${label}`} />
          </SelectTrigger>
          <SelectContent>
            {field.options.map((opt) => (
              <SelectItem key={opt} value={opt}>
                {opt}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <FieldError errors={[error]} />
      </Field>
    );
  }

  if (field.type === "multiselect" && field.options) {
    const selected = Array.isArray(value) ? (value as string[]) : [];
    return (
      <Field data-invalid={!!error}>
        <FieldLabel>
          {label}
          {field.required && <span className="text-destructive">*</span>}
        </FieldLabel>
        {field.description && (
          <FieldDescription>{field.description}</FieldDescription>
        )}
        <ToggleGroup
          type="multiple"
          value={selected}
          onValueChange={(vals) => onChange(vals)}
          className="flex flex-wrap justify-start gap-1.5"
        >
          {field.options.map((opt) => (
            <ToggleGroupItem key={opt} value={opt} size="sm">
              {opt}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
        <FieldError errors={[error]} />
      </Field>
    );
  }

  if (field.type === "textarea") {
    return (
      <Field data-invalid={!!error}>
        <FieldLabel htmlFor={fieldId}>
          {label}
          {field.required && <span className="text-destructive">*</span>}
        </FieldLabel>
        {field.description && (
          <FieldDescription>{field.description}</FieldDescription>
        )}
        <Textarea
          id={fieldId}
          aria-invalid={!!error}
          value={String(value ?? "")}
          onChange={(e) => onChange(e.target.value)}
          placeholder={field.placeholder ?? ""}
        />
        <FieldError errors={[error]} />
      </Field>
    );
  }

  if (field.type === "json") {
    const strValue = typeof value === "string" ? value : JSON.stringify(value ?? {}, null, 2);
    return (
      <Field data-invalid={!!error}>
        <FieldLabel>
          {label}
          {field.required && <span className="text-destructive">*</span>}
        </FieldLabel>
        {field.description && (
          <FieldDescription>{field.description}</FieldDescription>
        )}
        <CodeMirrorEditor
          value={strValue}
          onChange={(v) => {
            try {
              onChange(JSON.parse(v));
            } catch {
              onChange(v);
            }
          }}
          placeholder={field.placeholder ?? "{}"}
          minHeight="80px"
        />
        <FieldError errors={[error]} />
      </Field>
    );
  }

  return (
    <Field data-invalid={!!error}>
      <FieldLabel htmlFor={fieldId}>
        {label}
        {field.required && <span className="text-destructive">*</span>}
      </FieldLabel>
      {field.description && (
        <FieldDescription>{field.description}</FieldDescription>
      )}
      <Input
        id={fieldId}
        type={field.type === "number" ? "number" : "text"}
        aria-invalid={!!error}
        value={String(value ?? "")}
        onChange={(e) =>
          onChange(
            field.type === "number"
              ? e.target.value === "" ? 0 : Number(e.target.value)
              : e.target.value
          )
        }
        placeholder={field.placeholder ?? ""}
        className={field.type === "number" ? "font-mono" : ""}
      />
      <FieldError errors={[error]} />
    </Field>
  );
}

export function getFieldDefaults(
  fields: SchemaField[],
  existing?: unknown,
): Record<string, unknown> {
  const base =
    existing && typeof existing === "object" && !Array.isArray(existing)
      ? (existing as Record<string, unknown>)
      : {};
  const values: Record<string, unknown> = {};
  for (const field of fields) {
    if (field.name in base) {
      values[field.name] = base[field.name];
    } else if (field.default !== undefined) {
      values[field.name] = field.default;
    } else if (field.type === "boolean") {
      values[field.name] = false;
    } else if (field.type === "number") {
      values[field.name] = 0;
    } else if (field.type === "multiselect") {
      values[field.name] = [];
    } else {
      values[field.name] = "";
    }
  }
  return values;
}

interface JSONSchemaProperty {
  type?: string;
  title?: string;
  default?: unknown;
  enum?: string[];
  description?: string;
}

interface JSONSchemaObject {
  type?: string;
  properties?: Record<string, JSONSchemaProperty>;
  required?: string[];
}

export function jsonSchemaToFields(schema: unknown): SchemaField[] {
  if (!schema || typeof schema !== "object") return [];
  const s = schema as JSONSchemaObject;
  if (!s.properties || typeof s.properties !== "object") return [];

  const requiredSet = new Set(Array.isArray(s.required) ? s.required : []);
  const fields: SchemaField[] = [];

  for (const [name, prop] of Object.entries(s.properties)) {
    if (!prop || typeof prop !== "object") continue;

    let fieldType: SchemaField["type"] | null = null;

    if (prop.type === "string" && Array.isArray(prop.enum) && prop.enum.length > 0) {
      fieldType = "select";
    } else if (prop.type === "string") {
      fieldType = "string";
    } else if (prop.type === "number" || prop.type === "integer") {
      fieldType = "number";
    } else if (prop.type === "boolean") {
      fieldType = "boolean";
    } else if (prop.type === "object" || prop.type === "array") {
      fieldType = "json";
    }

    if (!fieldType) continue;

    fields.push({
      name,
      type: fieldType,
      label: prop.title ?? name,
      required: requiredSet.has(name),
      default: prop.default,
      options: prop.enum,
      description: prop.description,
    });
  }

  return fields;
}
