import PocketBase, { LocalAuthStore } from "pocketbase";
import { dataProvider, authProvider, liveProvider } from "refine-pocketbase";

const store = new LocalAuthStore("__pb_superuser_auth__");
const pb = new PocketBase("/", store);

export const pbClient = pb;
export const pbDataProvider = dataProvider(pb);
const baseAuthProvider = authProvider(pb, {
  collection: "_superusers",
});

export const pbAuthProvider = {
  ...baseAuthProvider,
  onError: async (error: any) => {
    if (error?.status === 401 || error?.status === 403) {
      pb.authStore.clear();
      return { redirectTo: "/login", logout: true, error };
    }
    return baseAuthProvider.onError(error);
  },
};
export const pbLiveProvider = liveProvider(pb);
