// Shared clipboard helper.
//
// navigator.clipboard is only available in secure contexts (HTTPS/localhost);
// fall back to a temporary textarea for plain-HTTP LAN access.
export function copyText(text: string) {
	if (navigator.clipboard && window.isSecureContext) {
		navigator.clipboard.writeText(text).catch(() => fallbackCopy(text));
	} else {
		fallbackCopy(text);
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
