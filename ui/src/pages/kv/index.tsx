import { useState, useMemo, useId } from "react";
import { useList, useInvalidate } from "@refinedev/core";
import { useMutation } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { pbClient } from "@/providers/pocketbase";
import { Database, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Field, FieldLabel } from "@/components/ui/field";
import { SchemaFormField, jsonSchemaToFields, getFieldDefaults, type SchemaField } from "@/components/schema-form";
import { CodeMirrorEditor } from "@/components/codemirror";
import { TableSkeleton } from "@/components/table-skeleton";
import { timeAgo } from "@/lib/format";
import type { PtKvResponse } from "@/types/pocketbase-types";

type KVRecord = PtKvResponse;

function truncateValue(value: unknown, maxLen = 60): string {
  const str = JSON.stringify(value) ?? "undefined";
  if (str.length <= maxLen) return str;
  return str.slice(0, maxLen) + "\u2026";
}

interface KVFormValues {
  key: string;
  value: string;
  schema: string;
  fieldValues: Record<string, unknown>;
  activeTab: string;
}

export function KVList() {
  const id = useId();
  const invalidate = useInvalidate();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editRecord, setEditRecord] = useState<KVRecord | null>(null);

  const { reset, watch, setValue, getValues } = useForm<KVFormValues>({
    defaultValues: {
      key: "",
      value: "",
      schema: "",
      fieldValues: {},
      activeTab: "value",
    },
  });

  const schema = watch("schema");
  const activeTab = watch("activeTab");
  const fieldValues = watch("fieldValues");
  const formValue = watch("value");
  const formKey = watch("key");

  const { result, query: kvQuery } = useList<KVRecord>({
    resource: "pt_kv",
    pagination: { mode: "off" },
  });

  const records = result.data ?? [];
  const isLoading = kvQuery.isLoading && records.length === 0;

  const { schemaFields, schemaError } = useMemo(() => {
    if (!schema.trim()) return { schemaFields: [] as SchemaField[], schemaError: "" };
    try {
      const parsed = JSON.parse(schema);
      return { schemaFields: jsonSchemaToFields(parsed), schemaError: "" };
    } catch {
      return { schemaFields: [] as SchemaField[], schemaError: "Invalid JSON" };
    }
  }, [schema]);

  const useSchemaForm = schemaFields.length > 0 && !schemaError;

  const openAdd = () => {
    reset();
    setEditRecord(null);
    setDialogOpen(true);
  };

  const openEdit = (record: KVRecord) => {
    setEditRecord(record);
    const schemaStr = record.schema ? JSON.stringify(record.schema, null, 2) : "";
    const fields = record.schema ? jsonSchemaToFields(record.schema) : [];
    reset({
      key: record.key,
      value: JSON.stringify(record.value, null, 2),
      schema: schemaStr,
      fieldValues: fields.length > 0 ? getFieldDefaults(fields, record.value) : {},
      activeTab: "value",
    });
    setDialogOpen(true);
  };

  const handleSchemaChange = (newSchema: string) => {
    setValue("schema", newSchema);
    if (!newSchema.trim()) return;
    try {
      const parsed = JSON.parse(newSchema);
      const fields = jsonSchemaToFields(parsed);
      if (fields.length > 0) {
        let existingValue: unknown = {};
        try {
          existingValue = JSON.parse(getValues("value"));
        } catch {
          // ignore
        }
        setValue("fieldValues", getFieldDefaults(fields, existingValue));
      }
    } catch {
      // schemaError is derived from useMemo
    }
  };

  const invalidateKV = () =>
    invalidate({ resource: "pt_kv", invalidates: ["list"] });

  const saveMutation = useMutation({
    mutationFn: async () => {
      const { key, value, schema: schemaVal, fieldValues: fv } = getValues();
      let parsed: unknown;
      if (useSchemaForm) {
        let extra: Record<string, unknown> = {};
        if (editRecord?.value && typeof editRecord.value === "object" && !Array.isArray(editRecord.value)) {
          extra = { ...(editRecord.value as Record<string, unknown>) };
        }
        parsed = { ...extra, ...fv };
      } else {
        parsed = JSON.parse(value);
      }

      let schemaToSave: unknown = null;
      if (schemaVal.trim()) {
        try {
          schemaToSave = JSON.parse(schemaVal);
        } catch {
          // Don't persist invalid schema
        }
      }

      const body: Record<string, unknown> = {
        value: parsed,
        schema: schemaToSave,
        updated_at_epoch_ms: Date.now(),
      };

      if (editRecord) {
        await pbClient.collection("pt_kv").update(editRecord.id, body);
      } else {
        await pbClient.collection("pt_kv").create({ key, ...body });
      }
    },
    onSuccess: () => {
      invalidateKV();
      toast.success(editRecord ? "Key updated" : "Key created");
      setDialogOpen(false);
    },
    onError: (err: any) => {
      toast.error(err?.message || "Failed to save");
    },
  });

  const onSubmit = () => {
    if (!useSchemaForm) {
      try {
        JSON.parse(getValues("value"));
      } catch {
        toast.error("Invalid JSON value");
        return;
      }
    }
    saveMutation.mutate();
  };

  const deleteMutation = useMutation({
    mutationFn: (recordId: string) =>
      pbClient.collection("pt_kv").delete(recordId),
    onSuccess: () => {
      invalidateKV();
      toast.success("Key deleted");
    },
    onError: (err: any) => {
      toast.error(err?.message || "Failed to delete");
    },
  });

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold">KV Store</h1>
        </div>
        <TableSkeleton columns={4} headers={["Key", "Value", "Updated", ""]} />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">KV Store</h1>
        <Button size="sm" onClick={openAdd}>
          <Plus className="mr-1.5 h-3.5 w-3.5" />
          Add Key
        </Button>
      </div>

      {kvQuery.isError && (
        <div className="rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {(kvQuery.error as unknown as Error)?.message ??
            "Failed to load KV data"}
        </div>
      )}

      {records.length === 0 && !kvQuery.isError ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 py-20 text-muted-foreground">
          <Database className="h-8 w-8" />
          <div className="text-center">
            <p>No keys stored.</p>
            <p className="mt-1 text-xs">
              Add a key-value pair using the button above or{" "}
              <code className="rounded bg-muted px-1.5 py-0.5 font-mono">
                turbine.KV.Set(ctx, key, value)
              </code>
            </p>
          </div>
        </div>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Key</TableHead>
                <TableHead>Value</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead className="w-10" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {records.map((record) => (
                <TableRow
                  key={record.id}
                  className="cursor-pointer"
                  onClick={() => openEdit(record)}
                >
                  <TableCell className="font-mono text-sm font-medium">
                    {record.key}
                  </TableCell>
                  <TableCell className="max-w-xs font-mono text-xs text-muted-foreground">
                    {truncateValue(record.value)}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {timeAgo(record.updated_at_epoch_ms)}
                  </TableCell>
                  <TableCell onClick={(e) => e.stopPropagation()}>
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                          aria-label={`Delete key ${record.key}`}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Delete key?</AlertDialogTitle>
                          <AlertDialogDescription>
                            This will permanently delete{" "}
                            <span className="font-mono">{record.key}</span>.
                            This cannot be undone.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction
                            variant="destructive"
                            onClick={() => deleteMutation.mutate(record.id)}
                          >
                            Delete
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-lg max-h-[85vh] flex flex-col overflow-hidden">
          <DialogHeader>
            <DialogTitle>{editRecord ? "Edit Key" : "Add Key"}</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4 overflow-hidden min-h-0">
            <Field className="shrink-0">
              <FieldLabel htmlFor="kv-key">Key</FieldLabel>
              <Input
                id="kv-key"
                value={formKey}
                onChange={(e) => setValue("key", e.target.value)}
                placeholder="my-key"
                className="font-mono"
                readOnly={!!editRecord}
                disabled={!!editRecord}
              />
            </Field>

            <Tabs value={activeTab} onValueChange={(v) => setValue("activeTab", v)} className="overflow-hidden min-h-0 flex flex-col">
              <TabsList className="shrink-0">
                <TabsTrigger value="value">Value</TabsTrigger>
                <TabsTrigger value="schema">Schema</TabsTrigger>
              </TabsList>

              <TabsContent value="value" className="overflow-y-auto min-h-0">
                {useSchemaForm ? (
                  <div className="space-y-3">
                    {schemaFields.map((field) => (
                      <SchemaFormField
                        key={field.name}
                        field={field}
                        value={fieldValues[field.name]}
                        onChange={(v) => {
                          const current = getValues("fieldValues");
                          setValue("fieldValues", { ...current, [field.name]: v });
                        }}
                        id={id}
                      />
                    ))}
                  </div>
                ) : (
                  <div className="space-y-1.5">
                    <CodeMirrorEditor
                      value={formValue}
                      onChange={(v) => setValue("value", v)}
                      placeholder="{}"
                      minHeight="120px"
                    />
                  </div>
                )}
              </TabsContent>

              <TabsContent value="schema" className="overflow-y-auto min-h-0">
                <div className="space-y-1.5">
                  <CodeMirrorEditor
                    value={schema}
                    onChange={handleSchemaChange}
                    placeholder='{"type":"object","properties":{"name":{"type":"string"}}}'
                    minHeight="80px"
                  />
                  {schemaError && (
                    <p className="text-xs text-destructive">{schemaError}</p>
                  )}
                  {schema.trim() && !schemaError && schemaFields.length === 0 && (
                    <p className="text-xs text-muted-foreground">No supported properties found in schema</p>
                  )}
                </div>
              </TabsContent>
            </Tabs>

            <div className="flex justify-end shrink-0">
              <Button
                onClick={() => onSubmit()}
                disabled={!formKey.trim() || saveMutation.isPending}
              >
                {saveMutation.isPending ? "Saving..." : "Save"}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
