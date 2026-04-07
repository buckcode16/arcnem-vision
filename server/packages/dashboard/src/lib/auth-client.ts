import { emailOTPClient, organizationClient } from "better-auth/client/plugins";
import { createAuthClient } from "better-auth/react";

export const authAPIBaseURL = import.meta.env.VITE_API_URL?.trim();

if (!authAPIBaseURL) {
	throw new Error("VITE_API_URL is not defined");
}

export const authClient = createAuthClient({
	baseURL: authAPIBaseURL,
	plugins: [emailOTPClient(), organizationClient()],
});

export const { signIn, signOut } = authClient;
