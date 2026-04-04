/**
* This file was @generated using pocketbase-typegen
*/

import type PocketBase from 'pocketbase'
import type { RecordService } from 'pocketbase'

export enum Collections {
	Authorigins = "_authOrigins",
	Externalauths = "_externalAuths",
	Mfas = "_mfas",
	Otps = "_otps",
	Superusers = "_superusers",
	PtAlertChannels = "pt_alert_channels",
	PtKv = "pt_kv",
	PtNotifications = "pt_notifications",
	PtOperationOutputs = "pt_operation_outputs",
	PtProducts = "pt_products",
	PtSchedules = "pt_schedules",
	PtWebhooks = "pt_webhooks",
	PtWorkflowEvents = "pt_workflow_events",
	PtWorkflowEventsHistory = "pt_workflow_events_history",
	PtWorkflowStatus = "pt_workflow_status",
	PtWorkflows = "pt_workflows",
	Users = "users",
}

// Alias types for improved usability
export type IsoDateString = string
export type IsoAutoDateString = string & { readonly autodate: unique symbol }
export type RecordIdString = string
export type FileNameString = string & { readonly filename: unique symbol }
export type HTMLString = string

type ExpandType<T> = unknown extends T
	? T extends unknown
		? { expand?: unknown }
		: { expand: T }
	: { expand: T }

// System fields
export type BaseSystemFields<T = unknown> = {
	id: RecordIdString
	collectionId: string
	collectionName: Collections
} & ExpandType<T>

export type AuthSystemFields<T = unknown> = {
	email: string
	emailVisibility: boolean
	username: string
	verified: boolean
} & BaseSystemFields<T>

// Record types for each collection

export type AuthoriginsRecord = {
	collectionRef: string
	created: IsoAutoDateString
	fingerprint: string
	id: string
	recordRef: string
	updated: IsoAutoDateString
}

export type ExternalauthsRecord = {
	collectionRef: string
	created: IsoAutoDateString
	id: string
	provider: string
	providerId: string
	recordRef: string
	updated: IsoAutoDateString
}

export type MfasRecord = {
	collectionRef: string
	created: IsoAutoDateString
	id: string
	method: string
	recordRef: string
	updated: IsoAutoDateString
}

export type OtpsRecord = {
	collectionRef: string
	created: IsoAutoDateString
	id: string
	password: string
	recordRef: string
	sentTo?: string
	updated: IsoAutoDateString
}

export type SuperusersRecord = {
	created: IsoAutoDateString
	email: string
	emailVisibility?: boolean
	id: string
	password: string
	tokenKey: string
	updated: IsoAutoDateString
	verified?: boolean
}

export type PtAlertChannelsRecord<Tevents = unknown> = {
	created: IsoAutoDateString
	enabled?: boolean
	events: null | Tevents
	id: string
	name: string
	url: string
}

export type PtKvRecord<Tschema = unknown, Tvalue = unknown> = {
	id: string
	key: string
	schema?: null | Tschema
	updated_at_epoch_ms?: number
	value: null | Tvalue
}

export type PtNotificationsRecord<Tmessage = unknown> = {
	consumed?: boolean
	created_at_epoch_ms?: number
	destination_id: RecordIdString
	id: string
	message?: null | Tmessage
	topic?: string
}

export type PtOperationOutputsRecord<Toutput = unknown> = {
	child_workflow_id?: string
	ended_at_epoch_ms?: number
	error?: string
	function_id: number
	function_name?: string
	id: string
	output?: null | Toutput
	started_at_epoch_ms?: number
	workflow_id: RecordIdString
}

export type PtProductsRecord<Tmetadata = unknown> = {
	created: IsoAutoDateString
	error?: string
	file?: FileNameString
	file_name: string
	function_id?: number
	function_name?: string
	id: string
	metadata?: null | Tmetadata
	size?: number
	status: string
	workflow_id: RecordIdString
}

export type PtSchedulesRecord<Tinput = unknown> = {
	created: IsoAutoDateString
	cron_expression?: string
	enabled?: boolean
	id: string
	input?: null | Tinput
	jitter?: string
	scheduled_at?: IsoDateString
	type: string
	workflow_fqn: string
}

export type PtWebhooksRecord<Tevents = unknown> = {
	created: IsoAutoDateString
	enabled?: boolean
	events: null | Tevents
	id: string
	secret?: string
	url: string
}

export type PtWorkflowEventsRecord<Tvalue = unknown> = {
	id: string
	key: string
	value?: null | Tvalue
	workflow_id: RecordIdString
}

export type PtWorkflowEventsHistoryRecord<Tvalue = unknown> = {
	function_id: number
	id: string
	key: string
	value?: null | Tvalue
	workflow_id: RecordIdString
}

export type PtWorkflowStatusRecord<Tinputs = unknown, Toutput = unknown, Ttags = unknown> = {
	app_status?: string
	app_status_color?: string
	application_id?: string
	application_version?: string
	created_at_epoch_ms?: number
	deduplication_id?: string
	error?: string
	executor_id: string
	forked_from_workflow_id?: RecordIdString
	id: string
	inputs?: null | Tinputs
	name: string
	output?: null | Toutput
	owner_xid?: string
	parent_function_id?: number
	parent_workflow_id?: RecordIdString
	priority?: number
	queue_name?: string
	queue_partition_key?: string
	recovery_attempts?: number
	status: string
	summary?: string
	tags?: null | Ttags
	updated_at_epoch_ms?: number
	workflow_deadline_epoch_ms?: number
	workflow_timeout_ms?: number
}

export type PtWorkflowsRecord<Tinput_schema = unknown, Ttags = unknown> = {
	cron_schedule?: string
	fqn: string
	id: string
	input_schema?: null | Tinput_schema
	name: string
	tags?: null | Ttags
	triggerable?: boolean
}

export type UsersRecord = {
	avatar?: FileNameString
	created: IsoAutoDateString
	email: string
	emailVisibility?: boolean
	id: string
	name?: string
	password: string
	tokenKey: string
	updated: IsoAutoDateString
	verified?: boolean
}

// Response types include system fields and match responses from the PocketBase API
export type AuthoriginsResponse<Texpand = unknown> = Required<AuthoriginsRecord> & BaseSystemFields<Texpand>
export type ExternalauthsResponse<Texpand = unknown> = Required<ExternalauthsRecord> & BaseSystemFields<Texpand>
export type MfasResponse<Texpand = unknown> = Required<MfasRecord> & BaseSystemFields<Texpand>
export type OtpsResponse<Texpand = unknown> = Required<OtpsRecord> & BaseSystemFields<Texpand>
export type SuperusersResponse<Texpand = unknown> = Required<SuperusersRecord> & AuthSystemFields<Texpand>
export type PtAlertChannelsResponse<Tevents = unknown, Texpand = unknown> = Required<PtAlertChannelsRecord<Tevents>> & BaseSystemFields<Texpand>
export type PtKvResponse<Tschema = unknown, Tvalue = unknown, Texpand = unknown> = Required<PtKvRecord<Tschema, Tvalue>> & BaseSystemFields<Texpand>
export type PtNotificationsResponse<Tmessage = unknown, Texpand = unknown> = Required<PtNotificationsRecord<Tmessage>> & BaseSystemFields<Texpand>
export type PtOperationOutputsResponse<Toutput = unknown, Texpand = unknown> = Required<PtOperationOutputsRecord<Toutput>> & BaseSystemFields<Texpand>
export type PtProductsResponse<Tmetadata = unknown, Texpand = unknown> = Required<PtProductsRecord<Tmetadata>> & BaseSystemFields<Texpand>
export type PtSchedulesResponse<Tinput = unknown, Texpand = unknown> = Required<PtSchedulesRecord<Tinput>> & BaseSystemFields<Texpand>
export type PtWebhooksResponse<Tevents = unknown, Texpand = unknown> = Required<PtWebhooksRecord<Tevents>> & BaseSystemFields<Texpand>
export type PtWorkflowEventsResponse<Tvalue = unknown, Texpand = unknown> = Required<PtWorkflowEventsRecord<Tvalue>> & BaseSystemFields<Texpand>
export type PtWorkflowEventsHistoryResponse<Tvalue = unknown, Texpand = unknown> = Required<PtWorkflowEventsHistoryRecord<Tvalue>> & BaseSystemFields<Texpand>
export type PtWorkflowStatusResponse<Tinputs = unknown, Toutput = unknown, Ttags = unknown, Texpand = unknown> = Required<PtWorkflowStatusRecord<Tinputs, Toutput, Ttags>> & BaseSystemFields<Texpand>
export type PtWorkflowsResponse<Tinput_schema = unknown, Ttags = unknown, Texpand = unknown> = Required<PtWorkflowsRecord<Tinput_schema, Ttags>> & BaseSystemFields<Texpand>
export type UsersResponse<Texpand = unknown> = Required<UsersRecord> & AuthSystemFields<Texpand>

// Types containing all Records and Responses, useful for creating typing helper functions

export type CollectionRecords = {
	_authOrigins: AuthoriginsRecord
	_externalAuths: ExternalauthsRecord
	_mfas: MfasRecord
	_otps: OtpsRecord
	_superusers: SuperusersRecord
	pt_alert_channels: PtAlertChannelsRecord
	pt_kv: PtKvRecord
	pt_notifications: PtNotificationsRecord
	pt_operation_outputs: PtOperationOutputsRecord
	pt_products: PtProductsRecord
	pt_schedules: PtSchedulesRecord
	pt_webhooks: PtWebhooksRecord
	pt_workflow_events: PtWorkflowEventsRecord
	pt_workflow_events_history: PtWorkflowEventsHistoryRecord
	pt_workflow_status: PtWorkflowStatusRecord
	pt_workflows: PtWorkflowsRecord
	users: UsersRecord
}

export type CollectionResponses = {
	_authOrigins: AuthoriginsResponse
	_externalAuths: ExternalauthsResponse
	_mfas: MfasResponse
	_otps: OtpsResponse
	_superusers: SuperusersResponse
	pt_alert_channels: PtAlertChannelsResponse
	pt_kv: PtKvResponse
	pt_notifications: PtNotificationsResponse
	pt_operation_outputs: PtOperationOutputsResponse
	pt_products: PtProductsResponse
	pt_schedules: PtSchedulesResponse
	pt_webhooks: PtWebhooksResponse
	pt_workflow_events: PtWorkflowEventsResponse
	pt_workflow_events_history: PtWorkflowEventsHistoryResponse
	pt_workflow_status: PtWorkflowStatusResponse
	pt_workflows: PtWorkflowsResponse
	users: UsersResponse
}

// Utility types for create/update operations

type ProcessCreateAndUpdateFields<T> = Omit<{
	// Omit AutoDate fields
	[K in keyof T as Extract<T[K], IsoAutoDateString> extends never ? K : never]: 
		// Convert FileNameString to File
		T[K] extends infer U ? 
			U extends (FileNameString | FileNameString[]) ? 
				U extends any[] ? File[] : File 
			: U
		: never
}, 'id'>

// Create type for Auth collections
export type CreateAuth<T> = {
	id?: RecordIdString
	email: string
	emailVisibility?: boolean
	password: string
	passwordConfirm: string
	verified?: boolean
} & ProcessCreateAndUpdateFields<T>

// Create type for Base collections
export type CreateBase<T> = {
	id?: RecordIdString
} & ProcessCreateAndUpdateFields<T>

// Update type for Auth collections
export type UpdateAuth<T> = Partial<
	Omit<ProcessCreateAndUpdateFields<T>, keyof AuthSystemFields>
> & {
	email?: string
	emailVisibility?: boolean
	oldPassword?: string
	password?: string
	passwordConfirm?: string
	verified?: boolean
}

// Update type for Base collections
export type UpdateBase<T> = Partial<
	Omit<ProcessCreateAndUpdateFields<T>, keyof BaseSystemFields>
>

// Get the correct create type for any collection
export type Create<T extends keyof CollectionResponses> =
	CollectionResponses[T] extends AuthSystemFields
		? CreateAuth<CollectionRecords[T]>
		: CreateBase<CollectionRecords[T]>

// Get the correct update type for any collection
export type Update<T extends keyof CollectionResponses> =
	CollectionResponses[T] extends AuthSystemFields
		? UpdateAuth<CollectionRecords[T]>
		: UpdateBase<CollectionRecords[T]>

// Type for usage with type asserted PocketBase instance
// https://github.com/pocketbase/js-sdk#specify-typescript-definitions

export type TypedPocketBase = {
	collection<T extends keyof CollectionResponses>(
		idOrName: T
	): RecordService<CollectionResponses[T]>
} & PocketBase
