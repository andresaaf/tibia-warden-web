<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { api, ApiError } from '$lib/api';
	import { currentUser, authLoading } from '$lib/stores';
	import { GroupRoom, type RoomEvent } from '$lib/ws';
	import type { Announcement, Creature, DiscordRole, Group, GroupMember, InviteCode } from '$lib/types';

	let groupId = $derived(Number($page.params.id));

	let group = $state<Group | null>(null);
	let announcements = $state<Announcement[]>([]);
	let creatures = $state<Creature[]>([]);
	let killedIds = $state<number[]>([]);
	let members = $state<GroupMember[]>([]);
	let invites = $state<InviteCode[]>([]);
	let loading = $state(true);
	let error = $state('');
	let room: GroupRoom | null = null;

	// New announcement form.
	let creatureId = $state<number | ''>('');
	let note = $state('');
	let posting = $state(false);
	let postError = $state('');
	let creatureQuery = $state('');
	let showCreatureList = $state(false);
	let highlightIndex = $state(0);

	let showAdmin = $state(false);
	let inviteMaxUses = $state(1);
	let discordCode = $state('');
	let discordBusy = $state(false);
	let discordRoles = $state<DiscordRole[]>([]);
	let showRoles = $state(false);
	let autodelete = $state(-1);

	$effect(() => {
		autodelete = group?.discordAutodeleteSeconds ?? -1;
	});

	let me = $derived($currentUser);
	let isManager = $derived(group?.role === 'owner' || group?.role === 'admin');

	$effect(() => {
		if (!$authLoading && !$currentUser) goto('/', { replaceState: true });
	});

	onMount(() => {
		init();
		return () => room?.close();
	});

	onDestroy(() => room?.close());

	async function init() {
		loading = true;
		error = '';
		try {
			group = await api.getGroup(groupId);
			const [ann, crt, killed] = await Promise.all([
				api.announcements(groupId),
				api.creatures('', []),
				api.killedCreatures()
			]);
			announcements = ann;
			creatures = crt;
			killedIds = killed;
			if (isManager) await loadAdminData();
			connectRoom();
		} catch (err) {
			if (err instanceof ApiError && err.status === 403) {
				error = 'You are not a member of this group.';
			} else if (err instanceof ApiError && err.status === 404) {
				error = 'Group not found.';
			} else {
				error = 'Failed to load the group.';
			}
		} finally {
			loading = false;
		}
	}

	async function loadAdminData() {
		try {
			[members, invites] = await Promise.all([api.members(groupId), api.invites(groupId)]);
		} catch {
			// non-fatal
		}
	}

	function connectRoom() {
		room = new GroupRoom(groupId, handleEvent);
		room.connect();
	}

	function handleEvent(event: RoomEvent) {
		if (event.type === 'announcement.created') {
			upsertAnnouncement(event.payload, true);
		} else if (event.type === 'announcement.updated') {
			upsertAnnouncement(event.payload, false);
		}
	}

	function upsertAnnouncement(a: Announcement, prepend: boolean) {
		const idx = announcements.findIndex((x) => x.id === a.id);
		if (idx >= 0) {
			announcements[idx] = a;
			announcements = [...announcements];
		} else if (prepend) {
			announcements = [a, ...announcements];
		}
	}

	let filteredCreatures = $derived.by(() => {
		const q = creatureQuery.trim().toLowerCase();
		if (!q) return creatures.slice(0, 8);
		return creatures
			.map((c) => ({ c, score: matchScore(c.name.toLowerCase(), q) }))
			.filter((x) => x.score < 4)
			.sort(
				(a, b) =>
					a.score - b.score ||
					a.c.name.length - b.c.name.length ||
					a.c.name.localeCompare(b.c.name)
			)
			.slice(0, 8)
			.map((x) => x.c);
	});

	// Lower score = better match: exact, prefix, word-start, then substring.
	function matchScore(name: string, q: string): number {
		if (name === q) return 0;
		if (name.startsWith(q)) return 1;
		if (name.split(/\s+/).some((w) => w.startsWith(q))) return 2;
		if (name.includes(q)) return 3;
		return 4;
	}

	function selectCreature(c: Creature) {
		creatureId = c.id;
		creatureQuery = c.name;
		showCreatureList = false;
	}

	function onCreatureInput() {
		// Typing invalidates any previous selection until a new one is chosen.
		creatureId = '';
		showCreatureList = true;
		highlightIndex = 0;
	}

	function onCreatureKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			showCreatureList = true;
			highlightIndex = Math.min(highlightIndex + 1, filteredCreatures.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			highlightIndex = Math.max(highlightIndex - 1, 0);
		} else if (e.key === 'Enter') {
			if (showCreatureList && filteredCreatures[highlightIndex]) {
				e.preventDefault();
				selectCreature(filteredCreatures[highlightIndex]);
			}
		} else if (e.key === 'Escape') {
			showCreatureList = false;
		}
	}

	async function postAnnouncement(e: SubmitEvent) {
		e.preventDefault();
		postError = '';
		if (!creatureId) {
			postError = 'Choose a creature.';
			return;
		}
		posting = true;
		try {
			await api.createAnnouncement(groupId, {
				creatureId: Number(creatureId),
				location: '',
				note: note.trim(),
				goldCost: 0
			});
			creatureId = '';
			note = '';
			creatureQuery = '';
			showCreatureList = false;
		} catch (err) {
			postError = err instanceof ApiError ? err.message : 'Failed to post.';
		} finally {
			posting = false;
		}
	}

	function myResponse(a: Announcement): string | null {
		return a.responses.find((r) => r.userId === me?.id)?.status ?? null;
	}
	function hasClaimed(a: Announcement): boolean {
		return a.claims.some((c) => c.userId === me?.id);
	}

	async function respond(a: Announcement, status: 'coming' | 'ready') {
		try {
			if (myResponse(a) === status) await api.clearResponse(a.id);
			else await api.setResponse(a.id, status);
		} catch {
			/* live update will reconcile */
		}
	}
	async function markKilled(a: Announcement) {
		try {
			await api.markAnnouncementKilled(a.id);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to mark killed.';
		}
	}
	async function claim(a: Announcement) {
		try {
			await api.claimAnnouncement(a.id);
			if (!killedIds.includes(a.creatureId)) killedIds = [...killedIds, a.creatureId];
		} catch {
			/* live update will reconcile */
		}
	}

	async function leave() {
		if (!confirm('Leave this group?')) return;
		try {
			await api.leaveGroup(groupId);
			goto('/groups');
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to leave group.';
		}
	}

	async function deleteGroup() {
		if (!group) return;
		const ok = confirm(
			`Delete the group "${group.name}"?\n\nThis permanently removes the group and all of its announcements for everyone. This cannot be undone.`
		);
		if (!ok) return;
		try {
			await api.deleteGroup(groupId);
			goto('/groups');
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to delete group.';
		}
	}

	async function createInvite() {
		try {
			await api.createInvite(groupId, 0, inviteMaxUses);
			invites = await api.invites(groupId);
		} catch {
			error = 'Failed to create invite.';
		}
	}

	function inviteExhausted(inv: InviteCode): boolean {
		return inv.maxUses !== null && inv.useCount >= inv.maxUses;
	}
	function inviteUsesLabel(inv: InviteCode): string {
		return inv.maxUses === null ? `∞ · ${inv.useCount} used` : `${inv.useCount}/${inv.maxUses} used`;
	}
	async function deleteInvite(inviteId: number) {
		try {
			await api.deleteInvite(groupId, inviteId);
			invites = await api.invites(groupId);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to delete invite.';
		}
	}
	async function setRole(userId: number, role: string) {
		try {
			await api.setRole(groupId, userId, role);
			members = await api.members(groupId);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to update role.';
		}
	}
	async function kick(userId: number) {
		if (!confirm('Remove this member?')) return;
		try {
			await api.removeMember(groupId, userId);
			members = await api.members(groupId);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to remove member.';
		}
	}

	function copyCode(code: string) {
		// navigator.clipboard is only available in secure contexts (HTTPS/localhost);
		// fall back to a temporary textarea for plain-HTTP LAN access.
		if (navigator.clipboard && window.isSecureContext) {
			navigator.clipboard.writeText(code).catch(() => fallbackCopy(code));
		} else {
			fallbackCopy(code);
		}
	}

	function fallbackCopy(text: string) {
		const ta = document.createElement('textarea');
		ta.value = text;
		ta.style.position = 'fixed';
		ta.style.opacity = '0';
		document.body.appendChild(ta);
		ta.focus();
		ta.select();
		try {
			document.execCommand('copy');
		} catch {
			/* ignore */
		}
		document.body.removeChild(ta);
	}

	async function generateDiscordCode() {
		discordBusy = true;
		try {
			const res = await api.createDiscordLinkCode(groupId);
			discordCode = res.code;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to create link code.';
		} finally {
			discordBusy = false;
		}
	}

	async function unlinkDiscord() {
		if (!confirm('Disconnect this group from its Discord channel?')) return;
		discordBusy = true;
		try {
			await api.unlinkDiscord(groupId);
			discordCode = '';
			group = await api.getGroup(groupId);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to unlink Discord.';
		} finally {
			discordBusy = false;
		}
	}

	async function openRolePicker() {
		discordBusy = true;
		try {
			discordRoles = await api.discordRoles(groupId);
			showRoles = true;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to load Discord roles.';
		} finally {
			discordBusy = false;
		}
	}

	async function pickRole(role: DiscordRole) {
		discordBusy = true;
		try {
			await api.setDiscordRole(groupId, role.id, role.name);
			showRoles = false;
			group = await api.getGroup(groupId);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to set role.';
		} finally {
			discordBusy = false;
		}
	}

	async function clearRole() {
		discordBusy = true;
		try {
			await api.clearDiscordRole(groupId);
			showRoles = false;
			group = await api.getGroup(groupId);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to clear role.';
		} finally {
			discordBusy = false;
		}
	}

	async function setAutodelete(seconds: number) {
		discordBusy = true;
		try {
			await api.setDiscordAutodelete(groupId, seconds);
			group = await api.getGroup(groupId);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to update setting.';
		} finally {
			discordBusy = false;
		}
	}

	function comingList(a: Announcement) {
		return a.responses.filter((r) => r.status === 'coming');
	}
	function readyList(a: Announcement) {
		return a.responses.filter((r) => r.status === 'ready');
	}
</script>

<div class="container">
	{#if loading}
		<p class="muted">Loading…</p>
	{:else if error && !group}
		<div class="card">
			<p class="error">{error}</p>
			<a class="btn" href="/groups">Back to groups</a>
		</div>
	{:else if group}
		<div class="spread header">
			<div>
				<a class="muted back" href="/groups">← Groups</a>
				<h1>{group.name}</h1>
				{#if group.description}<p class="muted">{group.description}</p>{/if}
			</div>
			<div class="row">
				{#if isManager}
					<button class="btn btn-sm" onclick={() => (showAdmin = !showAdmin)}>
						{showAdmin ? 'Hide' : 'Manage'}
					</button>
				{/if}
				{#if group.role !== 'owner'}
					<button class="btn btn-sm btn-danger" onclick={leave}>Leave</button>
				{/if}
			</div>
		</div>

		{#if error}<p class="error">{error}</p>{/if}

		{#if showAdmin && isManager}
			<div class="card stack admin">
				<div class="spread">
					<h3>Members</h3>
					<div class="row invite-create">
						<select bind:value={inviteMaxUses} aria-label="Invite uses">
							<option value={1}>Single use</option>
							<option value={5}>5 uses</option>
							<option value={25}>25 uses</option>
							<option value={0}>Unlimited</option>
						</select>
						<button class="btn btn-sm" onclick={createInvite}>+ Create invite</button>
					</div>
				</div>

				{#if invites.length}
					<div class="invites">
						{#each invites as inv (inv.id)}
							<div class="invite" class:used={inviteExhausted(inv)}>
								<code>{inv.code}</code>
								<span class="badge">{inviteUsesLabel(inv)}</span>
								{#if !inviteExhausted(inv)}
									<button class="btn btn-sm" onclick={() => copyCode(inv.code)}>Copy</button>
								{/if}
								<button class="btn-x" title="Delete invite" aria-label="Delete invite" onclick={() => deleteInvite(inv.id)}>×</button>
							</div>
						{/each}
					</div>
				{/if}

				<div class="stack member-list">
					{#each members as m (m.userId)}
						<div class="spread member">
							<div>
								<strong>{m.characterName || m.discordName}</strong>
								<span class="badge">{m.role}</span>
							</div>
							{#if m.role !== 'owner'}
								<div class="row">
									{#if group.role === 'owner'}
										{#if m.role === 'admin'}
											<button class="btn btn-sm" onclick={() => setRole(m.userId, 'member')}>
												Demote
											</button>
										{:else}
											<button class="btn btn-sm" onclick={() => setRole(m.userId, 'admin')}>
												Make admin
											</button>
										{/if}
									{/if}
									<button class="btn btn-sm btn-danger" onclick={() => kick(m.userId)}>Kick</button>
								</div>
							{/if}
						</div>
					{/each}
				</div>

				<div class="discord-section">
					<div class="spread">
						<h3>Discord</h3>
						{#if group.discordChannelId}
							<button class="btn btn-sm btn-danger" onclick={unlinkDiscord} disabled={discordBusy}>
								Disconnect
							</button>
						{/if}
					</div>
					{#if group.discordChannelId}
						<p class="muted small">
							✅ Connected. New announcements are mirrored to your Discord channel with interactive
							Coming / Ready / Killed buttons.
						</p>

						<div class="role-config">
							{#if group.discordRoleName}
								<span class="muted small">Mentions <span class="role-pill">@{group.discordRoleName}</span> on new posts.</span>
								<div class="row">
									<button class="btn btn-sm" onclick={openRolePicker} disabled={discordBusy}>Change</button>
									<button class="btn btn-sm" onclick={clearRole} disabled={discordBusy}>Clear</button>
								</div>
							{:else}
								<span class="muted small">No role pinged on announcements.</span>
								<button class="btn btn-sm" onclick={openRolePicker} disabled={discordBusy}>
									{discordBusy ? 'Loading…' : 'Set role to ping'}
								</button>
							{/if}
						</div>

						{#if showRoles}
							<div class="role-list">
								{#if discordRoles.length === 0}
									<div class="muted small" style="padding: 0.4rem 0.55rem">No roles found in this server.</div>
								{:else}
									{#each discordRoles as role (role.id)}
										<button type="button" class="role-opt" onclick={() => pickRole(role)} disabled={discordBusy}>
											<span
												class="role-dot"
												style={`background: ${role.color ? `#${role.color.toString(16).padStart(6, '0')}` : 'var(--text-dim)'}`}
											></span>
											<span>@{role.name}</span>
											{#if !role.mentionable}<span class="badge">not mentionable</span>{/if}
										</button>
									{/each}
								{/if}
							</div>
							<p class="muted small">
								If a role is “not mentionable”, either enable Allow anyone to @mention this role in Discord,
								or give the bot the Mention @everyone, @here, and All Roles permission.
							</p>
						{/if}

						<div class="role-config">
							<span class="muted small">Auto-delete the Discord post after a kill</span>
							<select
								bind:value={autodelete}
								onchange={() => setAutodelete(autodelete)}
								disabled={discordBusy}
							>
								<option value={-1}>Never</option>
								<option value={0}>Immediately</option>
								<option value={600}>After 10 minutes</option>
								<option value={3600}>After 1 hour</option>
								<option value={86400}>After 24 hours</option>
							</select>
						</div>
						<p class="muted small">The announcement always stays in the website feed.</p>
					{:else}
						<p class="muted small">
							Mirror announcements to a Discord channel. Invite the bot to your server, then generate
							a code below and run <code>/link &lt;code&gt;</code> in the target channel.
						</p>
						{#if discordCode}
							<div class="link-code">
								<code>/link {discordCode}</code>
								<button class="btn btn-sm" onclick={() => copyCode(`/link ${discordCode}`)}>Copy</button>
							</div>
							<p class="muted small">This code expires in 15 minutes and can be used once.</p>
						{:else}
							<button class="btn btn-sm" onclick={generateDiscordCode} disabled={discordBusy}>
								{discordBusy ? 'Generating…' : 'Generate link code'}
							</button>
						{/if}
					{/if}
				</div>

				{#if group.role === 'owner'}
					<div class="danger-zone">
						<div>
							<strong>Delete group</strong>
							<div class="muted small">
								Permanently removes this group and all of its announcements for everyone.
							</div>
						</div>
						<button class="btn btn-sm btn-danger" onclick={deleteGroup}>Delete group</button>
					</div>
				{/if}
			</div>
		{/if}

		<form class="card stack post-form" onsubmit={postAnnouncement}>
			<h3>Announce an Echo Warden</h3>
			<div class="combobox">
				<input
					type="text"
					placeholder="Search creature…"
					bind:value={creatureQuery}
					autocomplete="off"
					oninput={onCreatureInput}
					onfocus={() => (showCreatureList = true)}
					onblur={() => setTimeout(() => (showCreatureList = false), 120)}
					onkeydown={onCreatureKeydown}
				/>
				{#if showCreatureList && (filteredCreatures.length > 0 || creatureQuery.trim())}
					<div class="combobox-list">
						{#each filteredCreatures as c, i (c.id)}
							<button
								type="button"
								class="opt"
								class:highlight={i === highlightIndex}
								onclick={() => selectCreature(c)}
								onmousemove={() => (highlightIndex = i)}
							>
								<span class="opt-name">{c.name}</span>
								<span class="badge diff" data-diff={c.difficulty}>{c.difficulty}</span>
							</button>
						{/each}
						{#if filteredCreatures.length === 0}
							<div class="opt empty muted">No creatures match</div>
						{/if}
					</div>
				{/if}
			</div>
			<input type="text" placeholder="Note (optional)" bind:value={note} />
			{#if postError}<p class="error">{postError}</p>{/if}
			<button class="btn btn-primary" type="submit" disabled={posting}>
				{posting ? 'Posting…' : 'Post announcement'}
			</button>
		</form>

		<div class="stack feed">
			{#if announcements.length === 0}
				<p class="muted">No announcements yet. Be the first to spot an Echo Warden!</p>
			{/if}
			{#each announcements as a (a.id)}
				<div class="card announcement" class:killed={a.status === 'killed'}>
					<div class="spread">
						<div>
							<div class="row" style="gap: 0.5rem">
								<strong class="creature-name">{a.creatureName}</strong>
								{#if killedIds.includes(a.creatureId)}
									<span class="badge mine" title="You've already killed this Echo Warden">✓ In your list</span>
								{/if}
								{#if a.status === 'killed'}
									<span class="badge status-killed">Killed</span>
								{:else}
									<span class="badge status-open">Open</span>
								{/if}
							</div>
							{#if a.location}<div class="loc">📍 {a.location}</div>{/if}
							{#if a.note}<div class="muted note">{a.note}</div>{/if}
							<div class="muted small">
								by {a.authorName} · {new Date(a.createdAt).toLocaleTimeString()}
							</div>
						</div>
					</div>

					{#if a.status === 'open'}
						<div class="actions">
							<button
								class="btn btn-sm"
								class:on={myResponse(a) === 'coming'}
								onclick={() => respond(a, 'coming')}
							>
								🏃 Coming
							</button>
							<button
								class="btn btn-sm"
								class:on-ready={myResponse(a) === 'ready'}
								onclick={() => respond(a, 'ready')}
							>
								✅ Ready
							</button>
							{#if a.authorId === me?.id || isManager}
								<button class="btn btn-sm btn-danger" onclick={() => markKilled(a)}>
									💀 Killed
								</button>
							{/if}
						</div>

						<div class="responders">
							{#if comingList(a).length}
								<span class="muted small">Coming: {comingList(a).map((r) => r.characterName).join(', ')}</span>
							{/if}
							{#if readyList(a).length}
								<span class="muted small">Ready: {readyList(a).map((r) => r.characterName).join(', ')}</span>
							{/if}
						</div>
					{:else}
						<div class="actions">
							<button class="btn btn-sm btn-primary" disabled={hasClaimed(a)} onclick={() => claim(a)}>
								{hasClaimed(a) ? '✓ On your list' : '➕ I got it — tick my list'}
							</button>
						</div>
						<div class="responders">
							{#if a.claims.length}
								<span class="muted small">
									Got the kill: {a.claims.map((c) => c.characterName).join(', ')}
								</span>
							{:else}
								<span class="muted small">No one has claimed the benefit yet.</span>
							{/if}
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.header {
		align-items: flex-start;
		margin-bottom: 1rem;
	}
	.back {
		display: inline-block;
		margin-bottom: 0.3rem;
		font-size: 0.85rem;
	}
	.admin {
		margin-bottom: 1rem;
	}
	.invites {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
	.invite-create select {
		width: auto;
	}
	.invite {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		background: var(--bg-elev-2);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 0.3rem 0.5rem;
	}
	.invite.used code {
		text-decoration: line-through;
		color: var(--text-dim);
	}
	.invite code {
		font-family: ui-monospace, monospace;
		letter-spacing: 0.05em;
	}
	.btn-x {
		background: none;
		border: none;
		color: var(--danger);
		font-size: 1.15rem;
		line-height: 1;
		padding: 0 0.15rem;
		border-radius: 4px;
	}
	.btn-x:hover {
		background: var(--danger);
		color: #fff;
	}
	.member-list {
		gap: 0.4rem;
	}
	.member {
		padding: 0.4rem 0;
		border-bottom: 1px solid var(--border);
	}
	.discord-section {
		border-top: 1px solid var(--border);
		padding-top: 0.75rem;
		margin-top: 0.25rem;
	}
	.danger-zone {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		flex-wrap: wrap;
		border-top: 1px solid var(--danger);
		margin-top: 0.5rem;
		padding-top: 0.75rem;
	}
	.discord-section p {
		margin: 0.4rem 0;
	}
	.link-code {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		background: var(--bg-elev-2);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 0.4rem 0.6rem;
		width: fit-content;
	}
	.link-code code {
		font-family: ui-monospace, monospace;
		letter-spacing: 0.03em;
	}
	.role-config {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.6rem;
		flex-wrap: wrap;
		margin-top: 0.25rem;
	}
	.role-pill {
		color: var(--accent);
		font-weight: 600;
	}
	.role-config select {
		width: auto;
		max-width: 220px;
	}
	.role-list {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		margin-top: 0.5rem;
		max-height: 220px;
		overflow-y: auto;
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 4px;
	}
	.role-opt {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		width: 100%;
		background: none;
		border: none;
		color: var(--text);
		text-align: left;
		padding: 0.4rem 0.55rem;
		border-radius: 6px;
	}
	.role-opt:hover {
		background: var(--bg-elev);
	}
	.role-dot {
		width: 12px;
		height: 12px;
		border-radius: 50%;
		flex: none;
	}
	.post-form {
		margin-bottom: 1.25rem;
	}
	.combobox {
		position: relative;
	}
	.combobox-list {
		position: absolute;
		z-index: 20;
		top: calc(100% + 4px);
		left: 0;
		right: 0;
		background: var(--bg-elev-2);
		border: 1px solid var(--border);
		border-radius: 8px;
		box-shadow: var(--shadow);
		max-height: 260px;
		overflow-y: auto;
		padding: 4px;
	}
	.opt {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		width: 100%;
		background: none;
		border: none;
		color: var(--text);
		text-align: left;
		padding: 0.45rem 0.55rem;
		border-radius: 6px;
	}
	.opt.highlight {
		background: var(--bg-elev);
	}
	.opt.empty {
		cursor: default;
	}
	.opt-name {
		font-weight: 550;
	}
	.diff[data-diff='Harmless'] {
		color: var(--diff-harmless);
		border-color: var(--diff-harmless);
	}
	.diff[data-diff='Trivial'] {
		color: var(--diff-trivial);
		border-color: var(--diff-trivial);
	}
	.diff[data-diff='Easy'] {
		color: var(--diff-easy);
		border-color: var(--diff-easy);
	}
	.diff[data-diff='Medium'] {
		color: var(--diff-medium);
		border-color: var(--diff-medium);
	}
	.diff[data-diff='Hard'] {
		color: var(--diff-hard);
		border-color: var(--diff-hard);
	}
	.diff[data-diff='Challenging'] {
		color: var(--diff-challenging);
		border-color: var(--diff-challenging);
	}
	.feed {
		gap: 0.7rem;
	}
	.announcement.killed {
		border-color: var(--border);
		opacity: 0.92;
	}
	.creature-name {
		font-size: 1.1rem;
	}
	.loc {
		margin-top: 0.3rem;
	}
	.note {
		margin-top: 0.15rem;
	}
	.small {
		font-size: 0.82rem;
	}
	.actions {
		display: flex;
		gap: 0.4rem;
		margin-top: 0.75rem;
		flex-wrap: wrap;
	}
	.responders {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
		margin-top: 0.5rem;
	}
	.btn.on {
		border-color: var(--info);
		color: var(--info);
		background: color-mix(in srgb, var(--info) 15%, var(--bg-elev-2));
	}
	.btn.on-ready {
		border-color: var(--success);
		color: var(--success);
		background: color-mix(in srgb, var(--success) 15%, var(--bg-elev-2));
	}
	.status-open {
		color: var(--accent);
		border-color: var(--accent);
	}
	.status-killed {
		color: var(--danger);
		border-color: var(--danger);
	}
	.mine {
		color: var(--success);
		border-color: var(--success);
	}
</style>
