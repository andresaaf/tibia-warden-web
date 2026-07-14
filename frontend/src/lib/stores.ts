import { writable } from 'svelte/store';
import type { User } from './types';
import { api, ApiError } from './api';

/** The current authenticated user, or null when logged out. */
export const currentUser = writable<User | null>(null);

/** True until the initial auth check has completed. */
export const authLoading = writable(true);

/**
 * Loads the current user into the store. Returns the user, or null if the
 * request fails with 401 (not authenticated).
 */
export async function loadCurrentUser(): Promise<User | null> {
	authLoading.set(true);
	try {
		const user = await api.me();
		currentUser.set(user);
		return user;
	} catch (err) {
		if (err instanceof ApiError && err.status === 401) {
			currentUser.set(null);
			return null;
		}
		throw err;
	} finally {
		authLoading.set(false);
	}
}

export async function logout(): Promise<void> {
	await api.logout();
	currentUser.set(null);
}
