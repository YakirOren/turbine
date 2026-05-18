import {Authenticated, Refine} from "@refinedev/core";
import routerProvider, {CatchAllNavigate, NavigateToResource,} from "@refinedev/react-router";
import {BrowserRouter, Navigate, Outlet, Route, Routes} from "react-router";
import {pbAuthProvider, pbDataProvider, pbLiveProvider,} from "@/providers/pocketbase";
import {Layout} from "@/components/layout";
import {WorkflowList} from "@/pages/workflows/list";
import {WorkflowSteps} from "@/pages/workflows/steps";
import {QueueList} from "@/pages/queues/list";
import {ScheduledList} from "@/pages/scheduled/list";
import {LoginPage} from "@/pages/login";
import {KVList} from "@/pages/kv/index";
import {SettingsLayout} from "@/pages/settings/index";
import {WebhookList} from "@/pages/webhooks/index";
import {NotificationList} from "@/pages/notifications/index";
import {ProductList} from "@/pages/products/index";

function PaddedOutlet() {
    return (
        <div className="flex-1 overflow-y-auto bg-card p-6">
            <Outlet/>
        </div>
    );
}

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
                        meta: {label: "Workflows"},
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
                                fallback={<CatchAllNavigate to="/login"/>}
                            >
                                <Layout/>
                            </Authenticated>
                        }
                    >
                        <Route
                            index
                            element={
                                <NavigateToResource resource="pt_workflow_status"/>
                            }
                        />
                        <Route path="/workflows/:id/steps" element={<WorkflowSteps/>}/>
                        <Route path="/workflows" element={<WorkflowList/>}/>
                        <Route path="/queues" element={<QueueList/>}/>
                        <Route path="/products" element={<ProductList/>}/>
                        <Route element={<PaddedOutlet/>}>
                            <Route path="/scheduled" element={<ScheduledList/>}/>
                            <Route path="/kv" element={<KVList/>}/>
                            <Route path="/settings" element={<SettingsLayout/>}>
                                <Route index element={<Navigate to="webhooks" replace/>}/>
                                <Route path="webhooks" element={<WebhookList/>}/>
                                <Route path="notifications" element={<NotificationList/>}/>
                            </Route>
                        </Route>
                    </Route>

                    {/* Public routes, logged-in users redirected to dashboard */}
                    <Route
                        element={
                            <Authenticated key="auth-pages" fallback={<Outlet/>}>
                                <NavigateToResource resource="pt_workflow_status"/>
                            </Authenticated>
                        }
                    >
                        <Route path="/login" element={<LoginPage/>}/>
                    </Route>
                </Routes>
            </Refine>
        </BrowserRouter>
    );
}

export default App;
