import PocketBase, { LocalAuthStore } from "pocketbase";
import { dataProvider, authProvider, liveProvider } from "refine-pocketbase";

const store = new LocalAuthStore("__pb_superuser_auth__");
const pb = new PocketBase("/", store);

export const pbClient = pb;
export const pbDataProvider = dataProvider(pb);
export const pbAuthProvider = authProvider(pb, {
  collection: "_superusers",
});
export const pbLiveProvider = liveProvider(pb);
