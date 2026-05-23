-- Mock data for Turbine UI screenshots
-- Run: sqlite3 demo/pb_data/data.db < demo/mock_data.sql

-- Clear existing workflow data (keep registered workflows & schedules)
DELETE FROM pt_operation_outputs;
DELETE FROM pt_notifications;
DELETE FROM pt_workflow_events;
DELETE FROM pt_workflow_events_history;
DELETE FROM pt_products;
DELETE FROM pt_workflow_status;
DELETE FROM pt_webhooks;
DELETE FROM pt_alert_channels;
DELETE FROM pt_kv;

-- Base epoch: 2026-04-07 10:00:00 UTC = 1775646000000
-- We'll spread workflows across the last 7 days

--------------------------------------------------------------------------------
-- WORKFLOW INSTANCES
--------------------------------------------------------------------------------

-- === ORDER WORKFLOWS (mix of statuses, with app_status colors) ===

-- Order 1: SUCCESS - completed order (today, 2 min ago)
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, app_status, app_status_color, tags, summary)
VALUES ('ord_001', 'main.OrderWorkflow', 'SUCCESS',
  '{"order_id":"ORD-7841","customer":"Alice Chen"}',
  '{"charge_id":"charge_7841","stock_reserved":24,"tracking":"TRK-99281","invoice_id":"INV-7841"}',
  '', 'exec-1', 'v1.4.2', 'demo', 0,
  1775645880000, 1775645920000, '_pt_internal_queue',
  'fulfilled', 'green', '["orders","e2e"]', 'Order ORD-7841 for Alice Chen');

-- Order 2: SUCCESS - completed order (1h ago)
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, app_status, app_status_color, tags, summary)
VALUES ('ord_002', 'main.OrderWorkflow', 'SUCCESS',
  '{"order_id":"ORD-7840","customer":"Bob Martinez"}',
  '{"charge_id":"charge_7840","stock_reserved":3,"tracking":"TRK-99280","invoice_id":"INV-7840"}',
  '', 'exec-1', 'v1.4.2', 'demo', 0,
  1775642400000, 1775642440000, '_pt_internal_queue',
  'fulfilled', 'green', '["orders","e2e"]', 'Order ORD-7840 for Bob Martinez');

-- Order 3: ERROR - payment failed (30 min ago)
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, app_status, app_status_color, tags, summary)
VALUES ('ord_003', 'main.OrderWorkflow', 'ERROR',
  '{"order_id":"ORD-7842","customer":"Carol White"}',
  NULL,
  'payment gateway timeout: context deadline exceeded', 'exec-1', 'v1.4.2', 'demo', 3,
  1775644200000, 1775644260000, '_pt_internal_queue',
  'processing-payment', 'red', '["orders","e2e"]', 'Order ORD-7842 for Carol White');

-- Order 4: PENDING - just created (1 min ago)
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, app_status, app_status_color, tags, summary)
VALUES ('ord_004', 'main.OrderWorkflow', 'PENDING',
  '{"order_id":"ORD-7843","customer":"David Park"}',
  NULL, '', 'exec-1', 'v1.4.2', 'demo', 0,
  1775645940000, 1775645940000, '_pt_internal_queue',
  'validating', 'blue', '["orders","e2e"]', 'Order ORD-7843 for David Park');

-- Order 5: SUCCESS (yesterday)
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, app_status, app_status_color, tags, summary)
VALUES ('ord_005', 'main.OrderWorkflow', 'SUCCESS',
  '{"order_id":"ORD-7835","customer":"Emma Liu"}',
  '{"charge_id":"charge_7835","stock_reserved":12,"tracking":"TRK-99275","invoice_id":"INV-7835"}',
  '', 'exec-1', 'v1.4.1', 'demo', 0,
  1775559600000, 1775559640000, '_pt_internal_queue',
  'fulfilled', 'green', '["orders","e2e"]', 'Order ORD-7835 for Emma Liu');

-- Order 6: CANCELLED (yesterday)
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, app_status, app_status_color, tags, summary)
VALUES ('ord_006', 'main.OrderWorkflow', 'CANCELLED',
  '{"order_id":"ORD-7836","customer":"Frank Kim"}',
  NULL, 'cancelled by user', 'exec-1', 'v1.4.1', 'demo', 0,
  1775559900000, 1775560200000, '_pt_internal_queue',
  'cancelled', 'gray', '["orders","e2e"]', 'Order ORD-7836 for Frank Kim');

-- Order 7: MAX_RECOVERY_ATTEMPTS_EXCEEDED (2 days ago)
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, app_status, app_status_color, tags, summary)
VALUES ('ord_007', 'main.OrderWorkflow', 'MAX_RECOVERY_ATTEMPTS_EXCEEDED',
  '{"order_id":"ORD-7829","customer":"Grace Tanaka"}',
  NULL,
  'inventory service unreachable after 5 retries', 'exec-1', 'v1.4.1', 'demo', 5,
  1775473200000, 1775473800000, '_pt_internal_queue',
  'reserving-stock', 'red', '["orders","e2e"]', 'Order ORD-7829 for Grace Tanaka');

-- === DEPLOY WORKFLOWS ===

-- Deploy 1: WAITING_FOR_APPROVAL
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, app_status, app_status_color, tags, summary)
VALUES ('dep_001', 'deploy', 'WAITING_FOR_APPROVAL',
  '{"service":"api-gateway","environment":"production","regions":["us-east-1","eu-west-1"],"version":"v2.8.0","dry_run":false}',
  NULL, '', 'exec-1', 'v1.4.2', 'demo', 0,
  1775644800000, 1775645100000, '_pt_internal_queue',
  'awaiting approval', 'yellow', '["deploy","infra"]', '');

-- Deploy 2: SUCCESS (staging, today)
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, app_status, app_status_color, tags, summary)
VALUES ('dep_002', 'deploy', 'SUCCESS',
  '{"service":"api-gateway","environment":"staging","regions":["us-east-1"],"version":"v2.8.0","dry_run":false}',
  '{"deployed":true}',
  '', 'exec-1', 'v1.4.2', 'demo', 0,
  1775641800000, 1775642100000, '_pt_internal_queue',
  'deployed', 'green', '["deploy","infra"]', '');

-- Deploy 3: SUCCESS (dry run)
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, app_status, app_status_color, tags, summary)
VALUES ('dep_003', 'deploy', 'SUCCESS',
  '{"service":"payment-service","environment":"production","regions":["us-east-1","us-west-2","eu-west-1","ap-southeast-1"],"version":"v3.1.0","dry_run":true}',
  '{"deployed":true,"dry_run":true}',
  '', 'exec-1', 'v1.4.2', 'demo', 0,
  1775638200000, 1775638400000, '_pt_internal_queue',
  'deployed (dry run)', 'blue', '["deploy","infra"]', '');

-- === NOTIFY WORKFLOWS ===

-- Notify 1: SUCCESS urgent
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, app_status, app_status_color, tags, summary)
VALUES ('ntf_001', 'notify', 'SUCCESS',
  '{"channel":"incidents","message":"Database failover completed successfully. Primary restored.","urgent":true}',
  '{}', '', 'exec-1', 'v1.4.2', 'demo', 0,
  1775643600000, 1775643605000, '_pt_internal_queue',
  'sent', 'red', '["notifications"]', '');

-- Notify 2: SUCCESS normal
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, app_status, app_status_color, tags, summary)
VALUES ('ntf_002', 'notify', 'SUCCESS',
  '{"channel":"general","message":"Weekly metrics report generated","urgent":false}',
  '{}', '', 'exec-1', 'v1.4.2', 'demo', 0,
  1775640000000, 1775640003000, '_pt_internal_queue',
  'sent', 'green', '["notifications"]', '');

-- === EMAIL QUEUE WORKFLOWS (queue-based) ===

-- Email 1-8: Mix of statuses in the "emails" queue
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, deduplication_id, priority, tags, summary)
VALUES
  ('eml_001', 'main.EmailWorkflow', 'SUCCESS', '"alice@example.com"', '{}', '', 'exec-1', 'v1.4.2', 'demo', 0, 1775645700000, 1775645703000, 'emails', 'eml-001', 0, '["email","queue"]', ''),
  ('eml_002', 'main.EmailWorkflow', 'SUCCESS', '"bob@example.com"', '{}', '', 'exec-1', 'v1.4.2', 'demo', 0, 1775645710000, 1775645713000, 'emails', 'eml-002', 0, '["email","queue"]', ''),
  ('eml_003', 'main.EmailWorkflow', 'SUCCESS', '"carol@example.com"', '{}', '', 'exec-1', 'v1.4.2', 'demo', 0, 1775645720000, 1775645723000, 'emails', 'eml-003', 0, '["email","queue"]', ''),
  ('eml_004', 'main.EmailWorkflow', 'ERROR', '"invalid@"', NULL, 'invalid recipient address', 'exec-1', 'v1.4.2', 'demo', 1, 1775645730000, 1775645733000, 'emails', 'eml-004', 0, '["email","queue"]', ''),
  ('eml_005', 'main.EmailWorkflow', 'ENQUEUED', '"dave@example.com"', NULL, '', 'exec-1', 'v1.4.2', 'demo', 0, 1775645740000, 1775645740000, 'emails', 'eml-005', 0, '["email","queue"]', ''),
  ('eml_006', 'main.EmailWorkflow', 'ENQUEUED', '"eve@example.com"', NULL, '', 'exec-1', 'v1.4.2', 'demo', 0, 1775645750000, 1775645750000, 'emails', 'eml-006', 1, '["email","queue"]', ''),
  ('eml_007', 'main.EmailWorkflow', 'ENQUEUED', '"frank@example.com"', NULL, '', 'exec-1', 'v1.4.2', 'demo', 0, 1775645760000, 1775645760000, 'emails', 'eml-007', 0, '["email","queue"]', ''),
  ('eml_008', 'main.EmailWorkflow', 'SUCCESS', '"grace@example.com"', '{}', '', 'exec-1', 'v1.4.2', 'demo', 0, 1775645680000, 1775645683000, 'emails', 'eml-008', 0, '["email","queue"]', '');

-- === REPORT WORKFLOWS ===

INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, tags, summary)
VALUES
  ('rpt_001', 'main.ReportWorkflow', 'SUCCESS', '"weekly-revenue"', '"report generated"', '', 'exec-1', 'v1.4.2', 'demo', 0, 1775642400000, 1775642450000, '_pt_internal_queue', '["reports"]', ''),
  ('rpt_002', 'main.ReportWorkflow', 'SUCCESS', '"user-activity"', '"report generated"', '', 'exec-1', 'v1.4.2', 'demo', 0, 1775556000000, 1775556050000, '_pt_internal_queue', '["reports"]', '');

-- === SCHEDULED WORKFLOWS (MetricsSync + DailyCleanup) ===

-- MetricsSync runs every 5 min, show several recent ones
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, tags, summary)
VALUES
  ('sched_ms_01', 'main.MetricsSync', 'SUCCESS', '{}', '{"aggregated":true}', '', 'exec-1', 'v1.4.2', 'demo', 0, 1775645700000, 1775645704000, '_pt_internal_queue', '["metrics","scheduled"]', ''),
  ('sched_ms_02', 'main.MetricsSync', 'SUCCESS', '{}', '{"aggregated":true}', '', 'exec-1', 'v1.4.2', 'demo', 0, 1775645400000, 1775645404000, '_pt_internal_queue', '["metrics","scheduled"]', ''),
  ('sched_ms_03', 'main.MetricsSync', 'SUCCESS', '{}', '{"aggregated":true}', '', 'exec-1', 'v1.4.2', 'demo', 0, 1775645100000, 1775645104000, '_pt_internal_queue', '["metrics","scheduled"]', ''),
  ('sched_ms_04', 'main.MetricsSync', 'ERROR', '{}', NULL, 'metrics endpoint returned 503', 'exec-1', 'v1.4.2', 'demo', 1, 1775644800000, 1775644804000, '_pt_internal_queue', '["metrics","scheduled"]', '');

-- DailyCleanup
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, tags, summary)
VALUES
  ('sched_dc_01', 'main.DailyCleanup', 'SUCCESS', '{}', '{"pruned":142,"compacted":true}', '', 'exec-1', 'v1.4.2', 'demo', 0, 1775635200000, 1775635260000, '_pt_internal_queue', '["maintenance","scheduled"]', ''),
  ('sched_dc_02', 'main.DailyCleanup', 'SUCCESS', '{}', '{"pruned":89,"compacted":true}', '', 'exec-1', 'v1.4.1', 'demo', 0, 1775548800000, 1775548860000, '_pt_internal_queue', '["maintenance","scheduled"]', ''),
  ('sched_dc_03', 'main.DailyCleanup', 'SUCCESS', '{}', '{"pruned":201,"compacted":true}', '', 'exec-1', 'v1.4.1', 'demo', 0, 1775462400000, 1775462460000, '_pt_internal_queue', '["maintenance","scheduled"]', '');

-- === More historical data for calendar chart (past 7 days) ===
-- Day -3
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, tags, summary)
VALUES
  ('hist_01', 'main.OrderWorkflow', 'SUCCESS', '{"order_id":"ORD-7820","customer":"Hiro Nakamura"}', '{}', '', 'exec-1', 'v1.4.1', 'demo', 0, 1775386800000, 1775386840000, '_pt_internal_queue', '["orders","e2e"]', 'Order ORD-7820 for Hiro Nakamura'),
  ('hist_02', 'main.OrderWorkflow', 'SUCCESS', '{"order_id":"ORD-7821","customer":"Iris Chen"}', '{}', '', 'exec-1', 'v1.4.1', 'demo', 0, 1775390400000, 1775390440000, '_pt_internal_queue', '["orders","e2e"]', 'Order ORD-7821 for Iris Chen'),
  ('hist_03', 'main.OrderWorkflow', 'ERROR', '{"order_id":"ORD-7822","customer":"Jack Wilson"}', NULL, 'stock unavailable', 'exec-1', 'v1.4.1', 'demo', 2, 1775394000000, 1775394060000, '_pt_internal_queue', '["orders","e2e"]', 'Order ORD-7822 for Jack Wilson');

-- Day -4
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, tags, summary)
VALUES
  ('hist_04', 'main.OrderWorkflow', 'SUCCESS', '{"order_id":"ORD-7815","customer":"Kate Brown"}', '{}', '', 'exec-1', 'v1.4.0', 'demo', 0, 1775300400000, 1775300440000, '_pt_internal_queue', '["orders","e2e"]', 'Order ORD-7815 for Kate Brown'),
  ('hist_05', 'deploy', 'SUCCESS', '{"service":"user-service","environment":"staging","regions":["us-east-1"],"version":"v1.9.0","dry_run":false}', '{"deployed":true}', '', 'exec-1', 'v1.4.0', 'demo', 0, 1775304000000, 1775304300000, '_pt_internal_queue', '["deploy","infra"]', '');

INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, deduplication_id, priority, tags, summary)
VALUES
  ('hist_06', 'main.EmailWorkflow', 'SUCCESS', '"newsletter@example.com"', '{}', '', 'exec-1', 'v1.4.0', 'demo', 0, 1775307600000, 1775307603000, 'emails', 'eml-hist-06', 0, '["email","queue"]', '');

-- Day -5 & -6
INSERT INTO pt_workflow_status (id, name, status, inputs, output, error, executor_id, application_version, application_id, recovery_attempts, created_at_epoch_ms, updated_at_epoch_ms, queue_name, tags, summary)
VALUES
  ('hist_07', 'main.OrderWorkflow', 'SUCCESS', '{"order_id":"ORD-7810","customer":"Liam O''Brien"}', '{}', '', 'exec-1', 'v1.4.0', 'demo', 0, 1775214000000, 1775214040000, '_pt_internal_queue', '["orders","e2e"]', 'Order ORD-7810 for Liam O''Brien'),
  ('hist_08', 'main.OrderWorkflow', 'SUCCESS', '{"order_id":"ORD-7811","customer":"Mia Johansson"}', '{}', '', 'exec-1', 'v1.4.0', 'demo', 0, 1775127600000, 1775127640000, '_pt_internal_queue', '["orders","e2e"]', 'Order ORD-7811 for Mia Johansson'),
  ('hist_09', 'main.ReportWorkflow', 'SUCCESS', '"monthly-summary"', '"report generated"', '', 'exec-1', 'v1.4.0', 'demo', 0, 1775131200000, 1775131250000, '_pt_internal_queue', '["reports"]', '');

--------------------------------------------------------------------------------
-- OPERATION OUTPUTS (steps for key workflows)
--------------------------------------------------------------------------------

-- Order 1 (ord_001) - full successful order with parallel steps
INSERT INTO pt_operation_outputs (id, workflow_id, function_id, function_name, output, error, started_at_epoch_ms, ended_at_epoch_ms)
VALUES
  ('ord_001_0', 'ord_001', 0, 'validate', '"valid"', '', 1775645880000, 1775645882000),
  ('ord_001_1', 'ord_001', 1, 'charge', '"charge_7841"', '', 1775645882000, 1775645892000),
  ('ord_001_2', 'ord_001', 2, 'inventory', '24', '', 1775645882050, 1775645885000),
  ('ord_001_3', 'ord_001', 3, 'reserve', '{"reserved":true}', '', 1775645892000, 1775645895000),
  ('ord_001_4', 'ord_001', 4, 'invoice', '"INV-7841"', '', 1775645895000, 1775645900000),
  ('ord_001_5', 'ord_001', 5, 'ship', '"TRK-99281"', '', 1775645900000, 1775645915000),
  ('ord_001_6', 'ord_001', 6, 'confirm', '{"sent":true}', '', 1775645900050, 1775645903000);

-- Order 3 (ord_003) - failed at charge step
INSERT INTO pt_operation_outputs (id, workflow_id, function_id, function_name, output, error, started_at_epoch_ms, ended_at_epoch_ms)
VALUES
  ('ord_003_0', 'ord_003', 0, 'validate', '"valid"', '', 1775644200000, 1775644202000),
  ('ord_003_1', 'ord_003', 1, 'charge', NULL, 'payment gateway timeout: context deadline exceeded', 1775644202000, 1775644260000),
  ('ord_003_2', 'ord_003', 2, 'inventory', '18', '', 1775644202050, 1775644205000);

-- Deploy 1 (dep_001) - waiting for approval after build
INSERT INTO pt_operation_outputs (id, workflow_id, function_id, function_name, output, error, started_at_epoch_ms, ended_at_epoch_ms)
VALUES
  ('dep_001_0', 'dep_001', 0, 'build', '{"artifact":"api-gateway-v2.8.0.tar.gz","size_mb":42}', '', 1775644800000, 1775644860000);

-- Deploy 2 (dep_002) - completed staging deploy
INSERT INTO pt_operation_outputs (id, workflow_id, function_id, function_name, output, error, started_at_epoch_ms, ended_at_epoch_ms)
VALUES
  ('dep_002_0', 'dep_002', 0, 'build', '{"artifact":"api-gateway-v2.8.0.tar.gz","size_mb":42}', '', 1775641800000, 1775641860000),
  ('dep_002_1', 'dep_002', 1, 'approval', '{"approved_by":"admin","comment":"Staging looks good"}', '', 1775641860000, 1775641980000),
  ('dep_002_2', 'dep_002', 2, 'deploy', '{"deployed_to":["us-east-1"],"duration_s":120}', '', 1775641980000, 1775642100000);

-- MetricsSync (sched_ms_01)
INSERT INTO pt_operation_outputs (id, workflow_id, function_id, function_name, output, error, started_at_epoch_ms, ended_at_epoch_ms)
VALUES
  ('sched_ms_01_0', 'sched_ms_01', 0, 'fetch', '{"datapoints":1842}', '', 1775645700000, 1775645702000),
  ('sched_ms_01_1', 'sched_ms_01', 1, 'aggregate', '"aggregated"', '', 1775645702000, 1775645704000);

-- DailyCleanup (sched_dc_01)
INSERT INTO pt_operation_outputs (id, workflow_id, function_id, function_name, output, error, started_at_epoch_ms, ended_at_epoch_ms)
VALUES
  ('sched_dc_01_0', 'sched_dc_01', 0, 'prune', '{"deleted":142}', '', 1775635200000, 1775635230000),
  ('sched_dc_01_1', 'sched_dc_01', 1, 'compact', '{"compacted":true,"freed_mb":28}', '', 1775635230000, 1775635260000);

-- Email success (eml_001)
INSERT INTO pt_operation_outputs (id, workflow_id, function_id, function_name, output, error, started_at_epoch_ms, ended_at_epoch_ms)
VALUES
  ('eml_001_0', 'eml_001', 0, 'send', '{"delivered":true}', '', 1775645700000, 1775645703000);

-- Email error (eml_004)
INSERT INTO pt_operation_outputs (id, workflow_id, function_id, function_name, output, error, started_at_epoch_ms, ended_at_epoch_ms)
VALUES
  ('eml_004_0', 'eml_004', 0, 'send', NULL, 'invalid recipient address', 1775645730000, 1775645733000);

--------------------------------------------------------------------------------
-- NOTIFICATIONS (approval request)
--------------------------------------------------------------------------------

INSERT INTO pt_notifications (id, destination_id, topic, message, created_at_epoch_ms, consumed)
VALUES
  ('notif_001', 'dep_001', 'pt.approval', '{"requester":"deploy-bot","reason":"Production deploy of api-gateway v2.8.0 to us-east-1, eu-west-1"}', 1775644860000, 0);

--------------------------------------------------------------------------------
-- PRODUCTS (generated files)
--------------------------------------------------------------------------------

INSERT INTO pt_products (id, workflow_id, function_id, function_name, file_name, metadata, size, status, error, created)
VALUES
  ('prod_001', 'ord_001', 4, 'invoice', 'INV-7841.pdf', '{"invoice_id":"INV-7841","type":"invoice","customer":"Alice Chen"}', 48200, 'sent', '', '2026-04-07 09:58:20.000Z'),
  ('prod_002', 'ord_002', 4, 'invoice', 'INV-7840.pdf', '{"invoice_id":"INV-7840","type":"invoice","customer":"Bob Martinez"}', 47800, 'sent', '', '2026-04-07 09:00:40.000Z'),
  ('prod_003', 'ord_005', 4, 'invoice', 'INV-7835.pdf', '{"invoice_id":"INV-7835","type":"invoice","customer":"Emma Liu"}', 49100, 'sent', '', '2026-04-06 09:00:40.000Z'),
  ('prod_004', 'rpt_001', 0, 'generate', 'weekly-revenue-2026-W14.csv', '{"type":"revenue","period":"2026-W14"}', 125400, 'sent', '', '2026-04-07 09:00:50.000Z'),
  ('prod_005', 'rpt_002', 0, 'generate', 'user-activity-2026-W13.csv', '{"type":"activity","period":"2026-W13"}', 98200, 'sent', '', '2026-04-06 09:00:50.000Z');

--------------------------------------------------------------------------------
-- WORKFLOW EVENTS
--------------------------------------------------------------------------------

INSERT INTO pt_workflow_events (id, workflow_id, key, value)
VALUES
  ('evt_001', 'ord_001', 'payment_confirmed', '{"charge_id":"charge_7841","amount":149.99}'),
  ('evt_002', 'ord_001', 'shipping_label_created', '{"carrier":"FedEx","tracking":"TRK-99281"}'),
  ('evt_003', 'dep_001', 'build_completed', '{"artifact":"api-gateway-v2.8.0.tar.gz"}');

--------------------------------------------------------------------------------
-- SCHEDULES (keep existing compile ones, add user-created)
--------------------------------------------------------------------------------

DELETE FROM pt_schedules;

-- Compile-time schedules (from app registration)
INSERT INTO pt_schedules (id, workflow_fqn, input, type, cron_expression, jitter, scheduled_at, enabled)
VALUES
  ('sch_001', 'main.MetricsSync', NULL, 'compile', '*/5 * * * *', '', '', 1),
  ('sch_002', 'main.DailyCleanup', NULL, 'compile', '0 3 * * *', '', '', 1);

-- User-created cron schedule
INSERT INTO pt_schedules (id, workflow_fqn, input, type, cron_expression, jitter, scheduled_at, enabled)
VALUES
  ('sch_003', 'main.ReportWorkflow', '"weekly-revenue"', 'cron', '0 9 * * 1', '30s', '', 1);

-- One-time scheduled run
INSERT INTO pt_schedules (id, workflow_fqn, input, type, cron_expression, jitter, scheduled_at, enabled)
VALUES
  ('sch_004', 'main.ReportWorkflow', '"quarterly-audit"', 'once', '', '', '2026-04-15 09:00:00.000Z', 1);

-- Disabled schedule
INSERT INTO pt_schedules (id, workflow_fqn, input, type, cron_expression, jitter, scheduled_at, enabled)
VALUES
  ('sch_005', 'main.ReportWorkflow', '"daily-stats"', 'cron', '0 8 * * *', '', '', 0);

--------------------------------------------------------------------------------
-- WEBHOOKS
--------------------------------------------------------------------------------

INSERT INTO pt_webhooks (id, url, events, enabled, secret, created)
VALUES
  ('wh_001', 'https://hooks.slack.com/services/T0123/B0456/xyzabc', '["workflow.SUCCESS","workflow.ERROR"]', 1, 'whsec_a1b2c3d4e5f6', '2026-03-15 10:00:00.000Z'),
  ('wh_002', 'https://api.pagerduty.com/webhooks/v1/integration', '["workflow.ERROR","workflow.MAX_RECOVERY_ATTEMPTS_EXCEEDED"]', 1, 'whsec_pd_9876', '2026-03-20 14:30:00.000Z'),
  ('wh_003', 'https://internal.example.com/turbine/events', '["workflow.*"]', 0, '', '2026-04-01 08:00:00.000Z');

--------------------------------------------------------------------------------
-- ALERT CHANNELS
--------------------------------------------------------------------------------

INSERT INTO pt_alert_channels (id, name, url, events, enabled, created)
VALUES
  ('ac_001', 'Ops Slack', 'slack://xoxb-token/C0123CHANNEL', '["workflow.ERROR","workflow.MAX_RECOVERY_ATTEMPTS_EXCEEDED"]', 1, '2026-03-15 10:00:00.000Z'),
  ('ac_002', 'Deploy Notifications', 'discord://webhook-id/webhook-token', '["workflow.SUCCESS","workflow.WAITING_FOR_APPROVAL"]', 1, '2026-03-20 14:30:00.000Z'),
  ('ac_003', 'Email Alerts', 'smtp://user:pass@smtp.example.com:587/?from=alerts@example.com&to=team@example.com', '["workflow.ERROR"]', 1, '2026-04-01 08:00:00.000Z'),
  ('ac_004', 'Mobile Push', 'gotify://push.example.com/message?token=abc123', '["workflow.*"]', 0, '2026-04-05 12:00:00.000Z');

--------------------------------------------------------------------------------
-- KV STORE
--------------------------------------------------------------------------------

INSERT INTO pt_kv (id, key, value, updated_at_epoch_ms, schema)
VALUES
  ('kv_001', 'config/api', '{"api_url":"https://api.example.com","timeout_ms":5000,"max_retries":3,"environment":"production"}', 1775645000000,
   '{"type":"object","required":["api_url","environment"],"properties":{"api_url":{"type":"string","title":"API URL"},"timeout_ms":{"type":"integer","title":"Timeout (ms)","default":5000},"max_retries":{"type":"integer","title":"Max Retries","default":3},"environment":{"type":"string","title":"Environment","enum":["development","staging","production"]}}}'),
  ('kv_002', 'config/email', '{"smtp_host":"smtp.example.com","smtp_port":587,"from_address":"noreply@example.com","rate_limit":100}', 1775640000000, NULL),
  ('kv_003', 'feature-flags', '{"new_checkout":true,"dark_mode":true,"beta_reports":false,"maintenance_mode":false}', 1775644000000, NULL),
  ('kv_004', 'metrics/last-sync', '{"timestamp":"2026-04-07T09:55:00Z","datapoints":1842,"status":"ok"}', 1775645700000, NULL);
