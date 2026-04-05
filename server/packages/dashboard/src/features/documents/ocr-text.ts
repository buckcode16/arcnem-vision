const DEFAULT_CHUNK_SIZE = 1800;

function splitLongParagraph(paragraph: string, maxChars: number) {
	const chunks: string[] = [];
	let remaining = paragraph.trim();

	while (remaining.length > maxChars) {
		let splitAt = remaining.lastIndexOf(" ", maxChars);
		if (splitAt < Math.floor(maxChars * 0.55)) {
			splitAt = maxChars;
		}
		chunks.push(remaining.slice(0, splitAt).trim());
		remaining = remaining.slice(splitAt).trim();
	}

	if (remaining.length > 0) {
		chunks.push(remaining);
	}

	return chunks;
}

export function chunkOCRText(text: string, maxChars = DEFAULT_CHUNK_SIZE) {
	const normalized = text.trim();
	if (!normalized) {
		return [] as string[];
	}

	const paragraphs = normalized
		.split(/\n\s*\n/g)
		.map((paragraph) => paragraph.trim())
		.filter((paragraph) => paragraph.length > 0);

	if (paragraphs.length === 0) {
		return [normalized];
	}

	const chunks: string[] = [];
	let current = "";

	for (const paragraph of paragraphs) {
		if (paragraph.length > maxChars) {
			if (current) {
				chunks.push(current);
				current = "";
			}
			chunks.push(...splitLongParagraph(paragraph, maxChars));
			continue;
		}

		const next = current ? `${current}\n\n${paragraph}` : paragraph;
		if (next.length > maxChars) {
			if (current) {
				chunks.push(current);
			}
			current = paragraph;
			continue;
		}

		current = next;
	}

	if (current) {
		chunks.push(current);
	}

	return chunks.length > 0 ? chunks : [normalized];
}
