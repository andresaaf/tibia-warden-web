<script lang="ts">
	import { api, ApiError } from '$lib/api';
	import {
		WARDEN_FORMATS,
		type WardenFormat,
		type WardenImportMode,
		type WardenImportResult
	} from '$lib/types';

	let {
		open = $bindable(false),
		markedCount,
		onImported
	}: {
		open: boolean;
		/** How many wardens the user has marked (shown on the Export tab). */
		markedCount: number;
		/** Called after an import changed the Warden List. */
		onImported: () => void;
	} = $props();

	type Tab = 'export' | 'import';

	let dialog: HTMLDialogElement;
	let fileInput = $state<HTMLInputElement>(); // only mounted on the Import tab

	let tab = $state<Tab>('export');
	let format = $state<WardenFormat>(WARDEN_FORMATS[0].id);
	let busy = $state(false);
	let error = $state('');

	// Export
	let exportedFile = $state('');
	let unmapped = $state<string[]>([]);

	// Import: a picked file is previewed (dry run) before anything is applied.
	let mode = $state<WardenImportMode>('add');
	let fileName = $state('');
	let fileData: unknown = null;
	let preview = $state<WardenImportResult | null>(null);
	let applied = $state<WardenImportResult | null>(null);

	let formatLabel = $derived(WARDEN_FORMATS.find((f) => f.id === format)?.label ?? format);

	$effect(() => {
		if (open && !dialog.open) {
			reset();
			dialog.showModal();
		} else if (!open && dialog.open) {
			dialog.close();
		}
	});

	/** Every opening starts clean, so a stale preview (the list may have changed
	 * since) can never be confirmed. The chosen tab, format and mode are kept. */
	function reset() {
		error = '';
		exportedFile = '';
		unmapped = [];
		clearFile();
		applied = null;
	}

	/** Esc closes the dialog natively; keep `open` in sync. */
	function onClose() {
		open = false;
	}

	function onBackdropClick(e: MouseEvent) {
		if (e.target === dialog) open = false;
	}

	function setTab(next: Tab) {
		tab = next;
		error = '';
	}

	function setFormat(next: WardenFormat) {
		format = next;
		error = '';
		exportedFile = '';
		unmapped = [];
		clearFile();
	}

	function clearFile() {
		fileName = '';
		fileData = null;
		preview = null;
	}

	function message(err: unknown, fallback: string): string {
		return err instanceof ApiError ? `${fallback}: ${err.message}` : `${fallback}.`;
	}

	async function download() {
		busy = true;
		error = '';
		exportedFile = '';
		unmapped = [];
		try {
			const res = await api.exportWardens(format);
			const url = URL.createObjectURL(res.blob);
			const link = document.createElement('a');
			link.href = url;
			link.download = res.filename;
			link.click();
			setTimeout(() => URL.revokeObjectURL(url), 1000);
			exportedFile = res.filename;
			unmapped = res.unmapped;
		} catch (err) {
			error = message(err, 'Export failed');
		} finally {
			busy = false;
		}
	}

	async function onFilePicked(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = ''; // allow re-picking the same file
		if (!file) return;
		clearFile();
		applied = null;
		error = '';
		try {
			fileData = JSON.parse(await file.text());
		} catch {
			error = `${file.name} isn't a valid JSON file.`;
			return;
		}
		fileName = file.name;
		await runPreview();
	}

	async function setMode(next: WardenImportMode) {
		mode = next;
		applied = null;
		if (fileData !== null) await runPreview();
	}

	async function runPreview() {
		busy = true;
		error = '';
		try {
			preview = await api.importWardens(format, fileData, mode, true);
		} catch (err) {
			preview = null;
			error = message(err, "Couldn't read the file");
		} finally {
			busy = false;
		}
	}

	async function confirmImport() {
		busy = true;
		error = '';
		try {
			applied = await api.importWardens(format, fileData, mode, false);
			clearFile();
			onImported();
		} catch (err) {
			error = message(err, 'Import failed');
		} finally {
			busy = false;
		}
	}

	function wardens(n: number): string {
		return `${n} ${n === 1 ? 'warden' : 'wardens'}`;
	}

	/** Says exactly what confirming does, e.g. "Unmark 5 and add 42". */
	let confirmLabel = $derived.by(() => {
		if (!preview) return '';
		const { added, removed } = preview;
		if (removed && added) return `Unmark ${removed} and add ${added}`;
		if (removed) return `Unmark ${wardens(removed)}`;
		if (added) return `Add ${wardens(added)}`;
		return 'Nothing to change';
	});
</script>

<dialog
	bind:this={dialog}
	class="transfer-dialog"
	aria-labelledby="transfer-title"
	onclose={onClose}
	onclick={onBackdropClick}
>
	<div class="dialog-body stack">
		<div class="spread">
			<h2 id="transfer-title">Import / Export</h2>
			<button class="close" aria-label="Close" onclick={() => (open = false)}>✕</button>
		</div>

		<div class="segmented" role="group" aria-label="Import or export">
			<button
				class="segment"
				class:active={tab === 'export'}
				aria-pressed={tab === 'export'}
				onclick={() => setTab('export')}
			>
				Export
			</button>
			<button
				class="segment"
				class:active={tab === 'import'}
				aria-pressed={tab === 'import'}
				onclick={() => setTab('import')}
			>
				Import
			</button>
		</div>

		<fieldset class="options" disabled={busy}>
			<legend>Format</legend>
			{#each WARDEN_FORMATS as f}
				<label class="option" class:selected={format === f.id}>
					<input
						type="radio"
						name="transfer-format"
						checked={format === f.id}
						onchange={() => setFormat(f.id)}
					/>
					<span>
						<span class="option-title">{f.label}</span>
						<span class="option-hint">{f.hint}</span>
					</span>
				</label>
			{/each}
		</fieldset>

		{#if tab === 'export'}
			<p class="muted">
				Downloads your {wardens(markedCount)} marked on the Warden List as a {formatLabel} file.
			</p>
			<div class="actions">
				<button class="btn btn-primary" onclick={download} disabled={busy}>
					{busy ? 'Preparing…' : 'Download'}
				</button>
			</div>
			{#if exportedFile}
				<p class="ok">Saved {exportedFile}.</p>
				{#if unmapped.length}
					<div class="notice">
						<p>
							{wardens(unmapped.length)}
							{unmapped.length === 1 ? "isn't" : "aren't"} known to {formatLabel} and
							{unmapped.length === 1 ? 'was' : 'were'} left out:
						</p>
						<ul class="names">
							{#each unmapped as name}<li>{name}</li>{/each}
						</ul>
					</div>
				{/if}
			{/if}
		{:else}
			<fieldset class="options" disabled={busy}>
				<legend>Mode</legend>
				<label class="option" class:selected={mode === 'add'}>
					<input
						type="radio"
						name="import-mode"
						checked={mode === 'add'}
						onchange={() => setMode('add')}
					/>
					<span>
						<span class="option-title">Add only</span>
						<span class="option-hint">Marks the wardens in the file. Never unmarks anything.</span>
					</span>
				</label>
				<label class="option" class:selected={mode === 'replace'}>
					<input
						type="radio"
						name="import-mode"
						checked={mode === 'replace'}
						onchange={() => setMode('replace')}
					/>
					<span>
						<span class="option-title">Replace my list</span>
						<span class="option-hint">
							Makes your list match the file: also unmarks wardens the file doesn't list.
						</span>
					</span>
				</label>
			</fieldset>

			<div class="actions">
				<button class="btn" onclick={() => fileInput?.click()} disabled={busy}>
					{fileName ? 'Choose another file…' : `Choose ${formatLabel} file…`}
				</button>
				{#if fileName}<span class="muted file-name">{fileName}</span>{/if}
				<input
					bind:this={fileInput}
					type="file"
					accept=".json,application/json"
					hidden
					onchange={onFilePicked}
				/>
			</div>

			{#if preview}
				<div class="preview stack">
					<div class="counts">
						<span class="count add">+{preview.added} to add</span>
						{#if mode === 'replace'}
							<span class="count remove">−{preview.removed} to unmark</span>
						{/if}
						<span class="count">{preview.alreadyMarked} already marked</span>
						{#if preview.unknown}
							<span class="count">{preview.unknown} unrecognised</span>
						{/if}
					</div>
					{#if preview.keptUnsupported}
						<p class="muted small">
							{wardens(preview.keptUnsupported)} you've marked
							{preview.keptUnsupported === 1 ? "isn't" : "aren't"} known to {formatLabel}, so
							{preview.keptUnsupported === 1 ? 'it stays' : 'they stay'} marked.
						</p>
					{/if}
					{#if preview.removedNames.length}
						<div class="notice danger">
							<p>These will be unmarked:</p>
							<ul class="names">
								{#each preview.removedNames as name}<li>{name}</li>{/each}
							</ul>
						</div>
					{/if}
					{#if preview.addedNames.length}
						<details>
							<summary>Show the {wardens(preview.addedNames.length)} to add</summary>
							<ul class="names">
								{#each preview.addedNames as name}<li>{name}</li>{/each}
							</ul>
						</details>
					{/if}
					<div class="actions">
						<button
							class="btn"
							class:btn-primary={!preview.removed}
							class:btn-danger={preview.removed > 0}
							onclick={confirmImport}
							disabled={busy || (!preview.added && !preview.removed)}
						>
							{confirmLabel}
						</button>
						<button class="btn" onclick={clearFile} disabled={busy}>Cancel</button>
					</div>
				</div>
			{/if}

			{#if applied}
				<p class="ok">
					Imported from {formatLabel}: added {applied.added}{#if applied.mode === 'replace'}, unmarked
						{applied.removed}{/if}.
				</p>
			{/if}
		{/if}

		{#if error}<p class="error">{error}</p>{/if}
	</div>
</dialog>

<style>
	.transfer-dialog {
		width: min(34rem, calc(100% - 2rem));
		max-height: calc(100% - 2rem);
		padding: 0;
		background: var(--bg-elev);
		color: var(--text);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		box-shadow: var(--shadow);
	}
	.transfer-dialog::backdrop {
		background: rgba(0, 0, 0, 0.6);
	}
	.dialog-body {
		padding: 1.1rem 1.2rem 1.2rem;
	}
	h2 {
		margin: 0;
		font-size: 1.15rem;
	}
	p {
		margin: 0;
	}
	.close {
		background: transparent;
		border: none;
		color: var(--text-dim);
		font-size: 1rem;
		padding: 0.2rem 0.4rem;
	}
	.close:hover {
		color: var(--text);
	}
	.segmented {
		display: inline-flex;
		align-self: flex-start;
		background: var(--bg-elev-2);
		border: 1px solid var(--border);
		border-radius: 999px;
		padding: 2px;
	}
	.segment {
		border: none;
		background: transparent;
		color: var(--text-dim);
		border-radius: 999px;
		padding: 0.3rem 0.9rem;
		font-weight: 550;
		font-size: 0.85rem;
	}
	.segment.active {
		color: var(--text);
		background: color-mix(in srgb, var(--accent) 22%, var(--bg-elev-2));
		box-shadow: inset 0 0 0 1px var(--accent);
	}
	.options {
		border: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
	}
	legend {
		padding: 0;
		margin-bottom: 0.35rem;
		font-size: 0.8rem;
		font-weight: 600;
		color: var(--text-dim);
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	.option {
		display: flex;
		align-items: flex-start;
		gap: 0.6rem;
		padding: 0.55rem 0.7rem;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--bg-elev-2);
		cursor: pointer;
	}
	.option.selected {
		border-color: var(--accent);
		background: color-mix(in srgb, var(--accent) 10%, var(--bg-elev-2));
	}
	.option input {
		width: auto;
		margin: 0.2rem 0 0;
		accent-color: var(--accent);
	}
	.option-title {
		display: block;
		font-weight: 600;
	}
	.option-hint {
		display: block;
		font-size: 0.85rem;
		color: var(--text-dim);
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.6rem;
	}
	.file-name {
		font-size: 0.85rem;
		overflow-wrap: anywhere;
	}
	.preview {
		gap: 0.75rem;
		border-top: 1px solid var(--border);
		padding-top: 0.9rem;
	}
	.counts {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}
	.count {
		padding: 0.2rem 0.6rem;
		border: 1px solid var(--border);
		border-radius: 999px;
		font-size: 0.85rem;
		font-variant-numeric: tabular-nums;
		color: var(--text-dim);
	}
	.count.add {
		color: var(--success);
		border-color: var(--success);
	}
	.count.remove {
		color: var(--danger);
		border-color: var(--danger);
	}
	.notice {
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 0.6rem 0.75rem;
		font-size: 0.9rem;
	}
	.notice.danger {
		border-color: color-mix(in srgb, var(--danger) 60%, var(--border));
		background: color-mix(in srgb, var(--danger) 8%, var(--bg-elev));
	}
	.names {
		margin: 0.4rem 0 0;
		padding-left: 1.1rem;
		max-height: 10rem;
		overflow-y: auto;
		font-size: 0.85rem;
		columns: 2 12rem;
	}
	details {
		font-size: 0.9rem;
	}
	summary {
		cursor: pointer;
		color: var(--text-dim);
	}
	.small {
		font-size: 0.85rem;
	}
	.ok {
		color: var(--success);
	}
</style>
