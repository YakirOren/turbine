import { Refine, Authenticated } from "@refinedev/core";
import routerProvider, {
  NavigateToResource,
} from "@refinedev/react-router";
import { BrowserRouter, Routes, Route } from "react-router";

import {
  pbDataProvider,
  pbAuthProvider,
  pbLiveProvider,
} from "@/providers/pocketbase";
import { Layout } from "@/components/layout";
import { WorkflowList } from "@/pages/workflows/list";
import { QueueList } from "@/pages/queues/list";

// Redirect to PocketBase admin login (outside basename, so we use window.location)
function RedirectToAdmin() {
  window.location.href = "/_/";
  return null;
}

function App() {
  return (
    <BrowserRouter basename="/_/pocketflow">
      <Refine
        routerProvider={routerProvider}
        dataProvider={pbDataProvider}
        authProvider={pbAuthProvider}
        liveProvider={pbLiveProvider}
        resources={[
          {
            name: "pf_workflow_status",
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
          <Route
            element={
              <Authenticated
                key="authenticated-routes"
                fallback={<RedirectToAdmin />}
              >
                <Layout />
              </Authenticated>
            }
          >
            <Route
              index
              element={
                <NavigateToResource resource="pf_workflow_status" />
              }
            />
            <Route path="/workflows" element={<WorkflowList />} />
            <Route path="/queues" element={<QueueList />} />
          </Route>
        </Routes>
      </Refine>
    </BrowserRouter>
  );
}

export default App;
