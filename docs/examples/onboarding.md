# Onboarding

An event-driven onboarding flow in a single workflow. It starts when a user logs in, suspends until they create their first todo, pauses for a minute, and then finalizes.

**What to notice:**
- `app.OnRecordAuthRequest("users")` starts the workflow on login, `WithID("onboarding-"+userID)` makes later logins attach to the same run
- `turbine.Recv` suspends the workflow until the create-todo handler calls `turbine.Send`, there is no polling and no second workflow
- `turbine.Sleep` is durable, a restart during the minute sleeps only the time left
- The handler sends while the onboarding workflow is `PENDING`. Messages wait until `Recv` consumes them, so a todo created before the workflow reaches `Recv` is not lost, and only the first todo is read
- `turbine.SetValue` exposes the current stage, so the UI can render a stepper from `GET /onboarding`
- If no todo arrives within 24 hours, `Recv` returns the zero value and the workflow ends as expired

**Try it:**

```sh
go run ./examples/onboarding superuser upsert admin@example.com adminpass123
go run ./examples/onboarding serve
```

Create a user in the dashboard at `http://127.0.0.1:8090/_/`, then:

```sh
TOKEN=$(curl -s -X POST http://127.0.0.1:8090/api/collections/users/auth-with-password \
  -H 'content-type: application/json' \
  -d '{"identity":"user@example.com","password":"userpass123"}' | jq -r .token)

curl -s http://127.0.0.1:8090/onboarding -H "Authorization: $TOKEN"
# {"stage":"await_todo"}

curl -s -X POST http://127.0.0.1:8090/todos -H "Authorization: $TOKEN" \
  -H 'content-type: application/json' -d '{"text":"buy milk"}'

curl -s http://127.0.0.1:8090/onboarding -H "Authorization: $TOKEN"
# {"stage":"todo_captured"}, then {"stage":"done"} a minute later
```

<<< @/../examples/onboarding/main.go
