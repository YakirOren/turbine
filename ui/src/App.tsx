import { Refine, Authenticated } from "@refinedev/core";
import routerProvider, {
  NavigateToResource,
  CatchAllNavigate,
} from "@refinedev/react-router";
import { BrowserRouter, Routes, Route, Outlet } from "react-router";

import {
  pbDataProvider,
  pbAuthProvider,
  pbLiveProvider,
} from "@/providers/pocketbase";
import { Layout } from "@/components/layout";
import { WorkflowList } from "@/pages/workflows/list";
import { WorkflowSteps } from "@/pages/workflows/steps";
import { QueueList } from "@/pages/queues/list";
import { ScheduledList } from "@/pages/scheduled/list";
import { LoginPage } from "@/pages/login";

function App() {
  return (
    <BrowserRouter basename={import.meta.env.DEV ? "/" : "/_/turbine"}>
      <Refine
        routerProvider={routerProvider}
        dataProvider={pbDataProvider}
        authProvider={pbAuthProvider}
        liveProvider={pbLiveProvider}
        resources={[
          {
            name: "pt_workflow_status",
            list: "/workflows",
            meta: { label: "Workflows" },
          },
        ]}
        options={{
          syncWithLocation: true,
          liveMode: "auto",
        }}
      >
        <Routes>
          {/* Authenticated routes */}
          <Route
            element={
              <Authenticated
                key="authenticated-routes"
                fallback={<CatchAllNavigate to="/login" />}
              >
                <Layout />
              </Authenticated>
            }
          >
            <Route
              index
              element={
                <NavigateToResource resource="pt_workflow_status" />
              }
            />
            <Route path="/workflows" element={<WorkflowList />} />
            <Route path="/workflows/:id/steps" element={<WorkflowSteps />} />
            <Route path="/queues" element={<QueueList />} />
            <Route path="/scheduled" element={<ScheduledList />} />
          </Route>

          {/* Public routes — logged-in users redirected to dashboard */}
          <Route
            element={
              <Authenticated key="auth-pages" fallback={<Outlet />}>
                <NavigateToResource resource="pt_workflow_status" />
              </Authenticated>
            }
          >
            <Route path="/login" element={<LoginPage />} />
          </Route>
        </Routes>
      </Refine>
    </BrowserRouter>
  );
}

export default App;
