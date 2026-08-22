<script lang="ts">
	import type { Map as LeafletMap, TileLayer, CircleMarker } from 'leaflet';
	import { onMount } from 'svelte';

	// TibiaMap renders the Tibia world map (tiles from the public, MIT-licensed
	// tibia-map-data project on GitHub Pages) with L.CRS.Simple, where one in-game
	// coordinate equals one pixel. In 'pick' mode a click drops a single marker and
	// writes the coordinate back to x/y/z (and flips `touched`), so the caller only
	// stores a spot once the announcer actually interacts. In 'view' mode the map is
	// read-only and shows the marker at (x, y) on floor z.
	let {
		x = $bindable(),
		y = $bindable(),
		z = $bindable(),
		mode = 'view',
		touched = $bindable(false),
		height = '300px'
	}: {
		x: number | null;
		y: number | null;
		z: number | null;
		mode?: 'pick' | 'view';
		touched?: boolean;
		height?: string;
	} = $props();

	const TILE_BASE = 'https://tibiamaps.github.io/tibia-map-data/mapper';
	const MinZ = 0;
	const MaxZ = 15;
	// Sensible default centre (Thais temple) when nothing is marked yet.
	const DEFAULT = { x: 32369, y: 32241, z: 7 };
	// Current map floor being viewed; the marker's floor follows this in pick mode.
	let currentZ = $state(z ?? DEFAULT.z);

	let el: HTMLDivElement;
	let map: LeafletMap | undefined;
	let layer: TileLayer | undefined;
	let marker: CircleMarker | undefined;

	// Floor labels mirror Tibia's convention: 7 is ground ("0"), above is "+n".
	const floors = Array.from({ length: 16 }, (_, f) => ({
		value: f,
		label: f === 7 ? '0 (ground)' : f < 7 ? `+${7 - f}` : `-${f - 7}`
	}));

	onMount(() => {
		let destroyed = false;
		let cleanup: (() => void) | undefined;

		(async () => {
			const L = (await import('leaflet')).default;
			await import('leaflet/dist/leaflet.css');
			if (destroyed) return;

			const TibiaLayer = L.TileLayer.extend({
				getTileUrl(coords: { x: number; y: number }) {
					// At native zoom a tile spans 256 coords; its name uses the
					// absolute world coordinate of its top-left pixel.
					const fl = (this as unknown as { options: { floor: number } }).options.floor;
					return `${TILE_BASE}/Minimap_Color_${coords.x * 256}_${coords.y * 256}_${fl}.png`;
				}
			});

			// L.CRS.Simple negates Y (transformation 1,0,-1,0), which would make
			// tile coordinates negative and 404. Tibia's map uses plain +y, so use
			// a non-negating transformation: world (x, y) -> pixel (x, y) at zoom 0.
			const TibiaCRS = L.Util.extend({}, L.CRS.Simple, {
				transformation: new L.Transformation(1, 0, 1, 0)
			});

			map = L.map(el, {
				crs: TibiaCRS,
				minZoom: -2,
				maxZoom: 3,
				zoomControl: true,
				attributionControl: false,
				zoomSnap: 1
			});
			// World extent, so panning can't wander off the map. latLng = (y, x).
			map.setMaxBounds(
				L.latLngBounds([
					[30976, 31744],
					[33280, 34560]
				])
			);

			layer = new (TibiaLayer as unknown as typeof L.TileLayer)(TILE_BASE, {
				tileSize: 256,
				// Tiles exist only at native zoom 0; let Leaflet up/downscale them
				// across the map's whole zoom range. Without an explicit minZoom the
				// GridLayer default (0) clamps the map and breaks zooming out.
				minZoom: -2,
				maxZoom: 3,
				minNativeZoom: 0,
				maxNativeZoom: 0,
				noWrap: true,
				// eslint-disable-next-line @typescript-eslint/no-explicit-any
				floor: currentZ,
				// 1x1 transparent gif: hide 404s for tiles that don't exist.
				errorTileUrl:
					'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7'
			} as unknown as L.TileLayerOptions);
			layer.addTo(map);

			const cx = x ?? DEFAULT.x;
			const cy = y ?? DEFAULT.y;
			map.setView([cy, cx], 0);

			if (x != null && y != null) placeMarker(L, cy, cx);

			if (mode === 'pick') {
				map.on('click', (e: L.LeafletMouseEvent) => {
					const wx = Math.round(e.latlng.lng);
					const wy = Math.round(e.latlng.lat);
					x = wx;
					y = wy;
					z = currentZ;
					touched = true;
					placeMarker(L, wy, wx);
				});
			}

			// Shift + scroll changes floor instead of zooming. Capture on the wrapper
			// (an ancestor of Leaflet's container) so it runs before Leaflet's own
			// wheel-zoom handler and we can stop it.
			const wrapper = el.parentElement;
			const onWheel = (ev: WheelEvent) => {
				if (!ev.shiftKey) return;
				ev.preventDefault();
				ev.stopPropagation();
				const dir = ev.deltaY > 0 ? 1 : -1; // scroll down = go deeper (z+1)
				const next = Math.min(MaxZ, Math.max(MinZ, currentZ + dir));
				if (next !== currentZ) changeFloor(next);
			};
			wrapper?.addEventListener('wheel', onWheel, { capture: true, passive: false });

			cleanup = () => {
				wrapper?.removeEventListener('wheel', onWheel, { capture: true });
				map?.remove();
				map = undefined;
			};
		})();

		return () => {
			destroyed = true;
			cleanup?.();
		};
	});

	function placeMarker(L: typeof import('leaflet'), lat: number, lng: number) {
		if (!map) return;
		if (marker) {
			marker.setLatLng([lat, lng]);
		} else {
			marker = L.circleMarker([lat, lng], {
				radius: 7,
				color: '#fff',
				weight: 2,
				fillColor: '#dc2828',
				fillOpacity: 1
			}).addTo(map);
		}
	}

	function changeFloor(next: number) {
		currentZ = next;
		if (layer) {
			// eslint-disable-next-line @typescript-eslint/no-explicit-any
			(layer.options as unknown as { floor: number }).floor = next;
			layer.redraw();
		}
		// The marked spot belongs to whichever floor is being viewed when picked.
		if (mode === 'pick' && touched) z = next;
	}
</script>

<div class="tibia-map" style:height>
	<div class="canvas" bind:this={el}></div>
	<div class="floor">
		<label title="Shift + scroll to change floor">
			Floor
			<select value={currentZ} onchange={(e) => changeFloor(Number(e.currentTarget.value))}>
				{#each floors as f (f.value)}
					<option value={f.value}>{f.label}</option>
				{/each}
			</select>
		</label>
		{#if mode === 'pick'}
			<span class="hint">{touched ? `${x}, ${y}` : 'Click to mark · shift-scroll for floors'}</span>
		{:else}
			<span class="hint">Shift-scroll for floors</span>
		{/if}
	</div>
</div>

<style>
	.tibia-map {
		display: flex;
		flex-direction: column;
		/* Cap the width so the map keeps a near-square shape on wide desktop cards
		   instead of stretching into a short letterbox strip; on a narrow phone
		   card it just fills the available width. */
		max-width: 460px;
		border: 1px solid var(--border);
		border-radius: 8px;
		overflow: hidden;
		background: var(--bg-elev-2);
	}
	.canvas {
		flex: 1;
		min-height: 0;
		background: #181a1e;
	}
	/* Leaflet paints controls/tiles absolutely; keep them inside our rounded box. */
	.canvas :global(.leaflet-container) {
		background: #181a1e;
		font: inherit;
	}
	.floor {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		padding: 0.35rem 0.55rem;
		border-top: 1px solid var(--border);
		font-size: 0.85rem;
	}
	.floor label {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		color: var(--muted, var(--text));
	}
	.floor select {
		background: var(--bg-elev);
		color: var(--text);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 0.15rem 0.3rem;
	}
	.hint {
		color: var(--muted, var(--text));
	}
</style>
