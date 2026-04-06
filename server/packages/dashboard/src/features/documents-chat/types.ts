import { z } from "zod";

export const chatScopeSchema = z.object({
	kind: z.literal("organization"),
	organizationId: z.string().min(1),
	projectIds: z.array(z.string().min(1)).optional(),
	deviceIds: z.array(z.string().min(1)).optional(),
	documentIds: z.array(z.string().min(1)).optional(),
});

export type ChatScope = z.infer<typeof chatScopeSchema>;

export const documentChatCitationSchema = z.object({
	documentId: z.string().min(1),
	projectId: z.string().min(1),
	projectName: z.string().min(1),
	deviceId: z.string().min(1).nullable().optional(),
	deviceName: z.string().min(1).nullable().optional(),
	label: z.string().min(1),
	excerpt: z.string().min(1),
	matchReason: z.string().min(1),
});

export type DocumentChatCitation = z.infer<typeof documentChatCitationSchema>;

export const documentSearchMatchSchema = z.object({
	documentId: z.string().min(1),
	projectId: z.string().min(1),
	projectName: z.string().min(1),
	deviceId: z.string().min(1).nullable().optional(),
	deviceName: z.string().min(1).nullable().optional(),
	label: z.string().min(1),
	snippet: z.string().min(1),
	matchReason: z.string().min(1),
	score: z.number(),
	citation: documentChatCitationSchema,
});

export type DocumentSearchMatch = z.infer<typeof documentSearchMatchSchema>;

export const searchDocumentsInScopeOutputSchema = z.object({
	matches: z.array(documentSearchMatchSchema),
});

export type SearchDocumentsInScopeOutput = z.infer<
	typeof searchDocumentsInScopeOutputSchema
>;

export const documentOCRExcerptSchema = z.object({
	modelLabel: z.string().min(1),
	excerpt: z.string().min(1),
	createdAt: z.string().min(1),
});

export const documentSegmentationExcerptSchema = z.object({
	segmentationId: z.string().min(1),
	modelLabel: z.string().min(1),
	prompt: z.string().optional(),
	excerpt: z.string().min(1),
	createdAt: z.string().min(1),
});

export const documentContextItemSchema = z.object({
	documentId: z.string().min(1),
	projectId: z.string().min(1),
	projectName: z.string().min(1),
	deviceId: z.string().min(1).nullable().optional(),
	deviceName: z.string().min(1).nullable().optional(),
	label: z.string().min(1),
	description: z.string().optional(),
	ocrExcerpts: z.array(documentOCRExcerptSchema).default([]),
	segmentationExcerpts: z.array(documentSegmentationExcerptSchema).default([]),
	citation: documentChatCitationSchema,
});

export type DocumentContextItem = z.infer<typeof documentContextItemSchema>;

export const readDocumentContextOutputSchema = z.object({
	documents: z.array(documentContextItemSchema),
});

export type ReadDocumentContextOutput = z.infer<
	typeof readDocumentContextOutputSchema
>;

export const collectionSearcherResponseSchema = z.object({
	querySummary: z.string().min(1),
	documents: z
		.array(
			z.object({
				documentId: z.string().min(1),
				label: z.string().min(1),
				projectName: z.string().min(1),
				deviceName: z
					.string()
					.nullable()
					.describe("Device name when present, otherwise null."),
				matchReason: z.string().min(1),
				snippet: z.string().min(1),
				keyFacts: z.array(z.string().min(1)).max(5),
			}),
		)
		.max(6),
});

export type CollectionSearcherResponse = z.infer<
	typeof collectionSearcherResponseSchema
>;

export const assistantSourcesEventSchema = z.object({
	messageId: z.string().min(1),
	citations: z.array(documentChatCitationSchema),
});

export type AssistantSourcesEvent = z.infer<typeof assistantSourcesEventSchema>;
