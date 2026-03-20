import { extname } from "node:path";
import { createEnvVarGetter } from "@arcnem-vision/shared";
import { S3Client } from "bun";
import { sql } from "drizzle-orm";
import {
	agentGraphEdges,
	agentGraphNodes,
	agentGraphNodeTools,
	agentGraphRunSteps,
	agentGraphRuns,
	agentGraphs,
	apikeys,
	devices,
	documentDescriptionEmbeddings,
	documentDescriptions,
	documentEmbeddings,
	documents,
	members,
	models,
	organizations,
	projects,
	sessions,
	tools,
	users,
} from "./src/schema";
import { getDB } from "./src/server";

const db = getDB();
const now = new Date();
const plainPipelineApiKey =
	"seed_X9x8U7v6W5u4T3s2R1q0P9o8N7m6L5k4J3i2H1g0F9e8D7c6B5a4Z3y2X1w0";
const plainQualityReviewApiKey =
	"seed_Q7p6N5m4L3k2J1h0G9f8D7s6A5z4X3c2V1b0N9m8K7j6H5g4F3d2S1a0P9o8";
const plainSegmentationApiKey =
	"seed_seg_G4m2L8p1Q7r9S3t5U6v0W2x4Y8z1A3b5C7d9E0f2H4j6K8m0N2p4";
const seedDashboardSessionToken =
	"seed_dashboard_session_s4M8xR2vJ7nK1qP5wL9cD3fH6tY0uB4";
const seedDocumentEmbeddingDim = 768;
const semanticSegmentAnythingVersion =
	"b2691db53f2d96add0051a4a98e7a3861bd21bf5972031119d344d956d2f8256";
const langSegmentAnythingVersion =
	"891411c38a6ed2d44c004b7b9e44217df7a5b07848f29ddefd2e28bc7cbf93bc";

const S3_ENV_VAR = {
	S3_ACCESS_KEY_ID: "S3_ACCESS_KEY_ID",
	S3_SECRET_ACCESS_KEY: "S3_SECRET_ACCESS_KEY",
	S3_BUCKET: "S3_BUCKET",
	S3_ENDPOINT: "S3_ENDPOINT",
	S3_REGION: "S3_REGION",
} as const;

const getS3EnvVar = createEnvVarGetter(S3_ENV_VAR);

type SeedDocumentInput = {
	slug: string;
	fileName: string;
	description: string;
};

const seedDocumentInputs = [
	{
		slug: "mountain-vista",
		fileName: "mountain-vista.jpg",
		description:
			"Snowy mountain valley with rugged peaks, bright sky, and a broad view across the ridgeline.",
	},
	{
		slug: "vintage-car",
		fileName: "vintage-car.jpg",
		description:
			"Low-angle photograph of a vintage black car parked on a city street with warm sepia tones.",
	},
	{
		slug: "grazing-pigs",
		fileName: "grazing-pigs.jpg",
		description:
			"Two reddish-brown pigs grazing in bright green grass with a blue fence softly blurred behind them.",
	},
] as const satisfies ReadonlyArray<SeedDocumentInput>;

const seedQualityReviewDocumentInputs = [
	{
		slug: "soft-focus-cat",
		fileName: "soft-focus-cat.jpg",
		description:
			"Extreme close-up of a cat's nose with shallow depth of field that leaves most of the frame softly out of focus.",
	},
	{
		slug: "sharp-puppy",
		fileName: "sharp-puppy.jpg",
		description:
			"Sharp, well-exposed portrait of a black puppy sitting on weathered wooden planks and looking up at the camera.",
	},
	{
		slug: "washed-out-shoreline",
		fileName: "washed-out-shoreline.jpg",
		description:
			"Hazy shoreline scene with rocks in the foreground, pale sky, and muted contrast across the water.",
	},
] as const satisfies ReadonlyArray<SeedDocumentInput>;

const seedSegmentationDocumentInputs = [
	{
		slug: "vintage-car-segmentation",
		fileName: "vintage-car.jpg",
		description:
			"Vintage black car on a city street used for seeded semantic and language-prompted segmentation demos.",
	},
] as const satisfies ReadonlyArray<SeedDocumentInput>;

const semanticSegmentationInputSchema = {
	type: "object",
	properties: {
		image: { type: "string", description: "Input image" },
		output_json: { type: "boolean", default: true },
	},
	required: ["image"],
} as const;

const semanticSegmentationOutputSchema = {
	type: "object",
	title: "ModelOutput",
	properties: {
		img_out: { type: "string", format: "uri", title: "Img Out" },
		json_out: { type: "string", format: "uri", title: "Json Out" },
	},
	required: ["img_out"],
} as const;

const semanticSegmentationModelConfig = {
	input_image_field: "image",
	input_defaults: {
		output_json: true,
	},
	result_path: "json_out",
	result_source: "url_json",
	output_image_path: "img_out",
} as const;

const langSegmentationInputSchema = {
	type: "object",
	properties: {
		image: { type: "string", description: "Path to the input image" },
		text_prompt: {
			type: "string",
			description: "Text prompt for segmentation",
		},
	},
	required: ["image", "text_prompt"],
} as const;

const langSegmentationOutputSchema = {
	type: "string",
	format: "uri",
	title: "Output",
} as const;

const langSegmentationModelConfig = {
	input_image_field: "image",
	input_defaults: {},
	result_path: "$",
	result_source: "raw",
	output_image_path: "$",
} as const;

type UploadedSeedDocument = {
	slug: string;
	objectKey: string;
	contentType: string;
	eTag: string;
	sizeBytes: number;
	lastModifiedAt: Date;
	description: string;
};

const getSeedImageContentType = (fileName: string): string => {
	switch (extname(fileName).toLowerCase()) {
		case ".jpg":
		case ".jpeg":
			return "image/jpeg";
		case ".png":
			return "image/png";
		case ".webp":
			return "image/webp";
		default:
			throw new Error(`Unsupported seed image extension: ${fileName}`);
	}
};

const hashApiKey = async (key: string): Promise<string> => {
	const digest = await crypto.subtle.digest(
		"SHA-256",
		new TextEncoder().encode(key),
	);

	return Buffer.from(digest).toString("base64url");
};

const createDeterministicEmbedding = (
	seed: string,
	dim = seedDocumentEmbeddingDim,
): number[] => {
	let state = 2166136261;
	for (const character of seed) {
		state ^= character.charCodeAt(0);
		state = Math.imul(state, 16777619) >>> 0;
	}

	const embedding = new Array<number>(dim);
	for (let index = 0; index < dim; index += 1) {
		state = (Math.imul(state, 1664525) + 1013904223) >>> 0;
		const normalized = (state / 4294967295) * 2 - 1;
		embedding[index] = Number(normalized.toFixed(6));
	}

	return embedding;
};

const getSeedS3Client = (): { bucket: string; client: S3Client } => {
	const bucket = getS3EnvVar("S3_BUCKET");

	return {
		bucket,
		client: new S3Client({
			accessKeyId: getS3EnvVar("S3_ACCESS_KEY_ID"),
			secretAccessKey: getS3EnvVar("S3_SECRET_ACCESS_KEY"),
			bucket,
			endpoint: getS3EnvVar("S3_ENDPOINT"),
			region: getS3EnvVar("S3_REGION"),
		}),
	};
};

const uploadSeedDocumentsToS3 = async (
	s3Client: S3Client,
	inputs: ReadonlyArray<SeedDocumentInput>,
	pathPrefix: string,
): Promise<UploadedSeedDocument[]> => {
	return Promise.all(
		inputs.map(async (seedDocumentInput) => {
			const imageExtension = extname(seedDocumentInput.fileName).toLowerCase();
			if (imageExtension.length === 0) {
				throw new Error(
					`Seed image is missing an extension: ${seedDocumentInput.fileName}`,
				);
			}

			const sourceFile = Bun.file(
				new URL(`./seed-images/${seedDocumentInput.fileName}`, import.meta.url),
			);
			if (!(await sourceFile.exists())) {
				throw new Error(`Missing seed image: ${seedDocumentInput.fileName}`);
			}

			const sourceContentType =
				sourceFile.type.trim().toLowerCase() ||
				getSeedImageContentType(seedDocumentInput.fileName);
			const sourceImage = Buffer.from(await sourceFile.arrayBuffer());
			if (sourceImage.byteLength === 0) {
				throw new Error(`Seed image is empty: ${seedDocumentInput.fileName}`);
			}

			const objectKey = `${pathPrefix}/${seedDocumentInput.slug}${imageExtension}`;
			await Bun.write(s3Client.file(objectKey), sourceImage);
			const stats = await s3Client.stat(objectKey);

			const lastModifiedAt =
				stats.lastModified instanceof Date &&
				!Number.isNaN(stats.lastModified.getTime())
					? stats.lastModified
					: new Date();
			const eTag =
				typeof stats.etag === "string" && stats.etag.length > 0
					? stats.etag
					: `seed-${seedDocumentInput.slug}`;
			const contentType =
				typeof stats.type === "string" && stats.type.length > 0
					? stats.type.toLowerCase()
					: sourceContentType;

			return {
				slug: seedDocumentInput.slug,
				objectKey,
				contentType,
				eTag,
				sizeBytes:
					Number.isInteger(stats.size) && stats.size > 0
						? stats.size
						: sourceImage.byteLength,
				lastModifiedAt,
				description: seedDocumentInput.description,
			};
		}),
	);
};

const seed = async () => {
	const hashedPipelineApiKey = await hashApiKey(plainPipelineApiKey);
	const hashedQualityReviewApiKey = await hashApiKey(plainQualityReviewApiKey);
	const hashedSegmentationApiKey = await hashApiKey(plainSegmentationApiKey);
	const { bucket, client: s3Client } = getSeedS3Client();
	const uploadedSeedDocuments = await uploadSeedDocumentsToS3(
		s3Client,
		seedDocumentInputs,
		"seed/documents",
	);
	const uploadedQualityReviewDocuments = await uploadSeedDocumentsToS3(
		s3Client,
		seedQualityReviewDocumentInputs,
		"seed/quality-review",
	);
	const uploadedSegmentationDocuments = await uploadSeedDocumentsToS3(
		s3Client,
		seedSegmentationDocumentInputs,
		"seed/segmentations",
	);

	const result = await db.transaction(async (tx) => {
		await tx.execute(sql`
			TRUNCATE TABLE
				"agent_graph_run_steps",
				"agent_graph_runs",
				"agent_graph_edges",
				"agent_graph_node_tools",
				"agent_graph_nodes",
				"agent_graphs",
				"tools",
				"document_segmentations",
				"document_description_embeddings",
				"document_descriptions",
				"document_embeddings",
				"documents",
				"presigned_uploads",
				"models",
				"apikeys",
				"invitations",
				"members",
				"devices",
				"projects",
				"accounts",
				"sessions",
				"verifications",
				"organizations",
				"users"
			RESTART IDENTITY CASCADE
		`);

		// ── Auth / Org / Project ──

		const [user] = await tx
			.insert(users)
			.values({
				name: "Seed User",
				email: "seed.user@arcnem.local",
				emailVerified: true,
				role: "admin",
			})
			.returning({
				id: users.id,
				email: users.email,
			});
		if (!user) throw new Error("Failed to create seed user");

		const [organization] = await tx
			.insert(organizations)
			.values({
				name: "Seed Organization",
				slug: "seed-organization",
				createdAt: now,
			})
			.returning({
				id: organizations.id,
				slug: organizations.slug,
			});
		if (!organization) throw new Error("Failed to create seed organization");

		await tx.insert(members).values({
			organizationId: organization.id,
			userId: user.id,
			role: "owner",
			createdAt: now,
		});

		const sessionExpiresAt = new Date(now.getTime() + 1000 * 60 * 60 * 24 * 30);

		const [dashboardSession] = await tx
			.insert(sessions)
			.values({
				token: seedDashboardSessionToken,
				userId: user.id,
				expiresAt: sessionExpiresAt,
				ipAddress: "127.0.0.1",
				userAgent: "seed-dashboard-session",
				activeOrganizationId: organization.id,
				createdAt: now,
				updatedAt: now,
			})
			.returning({
				id: sessions.id,
				token: sessions.token,
				expiresAt: sessions.expiresAt,
			});
		if (!dashboardSession)
			throw new Error("Failed to create seed dashboard session");

		const [project] = await tx
			.insert(projects)
			.values({
				name: "Seed Project",
				slug: "seed-project",
				organizationId: organization.id,
			})
			.returning({
				id: projects.id,
				slug: projects.slug,
			});
		if (!project) throw new Error("Failed to create seed project");

		// ── Models ──

		const [clipModel] = await tx
			.insert(models)
			.values({
				provider: "REPLICATE",
				name: "openai/clip",
				version: "",
				type: "embedding",
				embeddingDim: 768,
				config: {},
			})
			.returning({ id: models.id });
		if (!clipModel) throw new Error("Failed to create CLIP model");

		const [gpt41MiniModel] = await tx
			.insert(models)
			.values({
				provider: "OPENAI",
				name: "gpt-4.1-mini",
				version: "",
				type: "chat",
				config: {},
			})
			.returning({ id: models.id });
		if (!gpt41MiniModel) throw new Error("Failed to create GPT-4.1-mini model");

		const [semanticSegmentationModel] = await tx
			.insert(models)
			.values({
				provider: "REPLICATE",
				name: "cjwbw/semantic-segment-anything",
				version: semanticSegmentAnythingVersion,
				type: "segmentation",
				inputSchema: semanticSegmentationInputSchema,
				outputSchema: semanticSegmentationOutputSchema,
				config: semanticSegmentationModelConfig,
			})
			.returning({ id: models.id });
		if (!semanticSegmentationModel) {
			throw new Error("Failed to create semantic segmentation model");
		}

		const [langSegmentationModel] = await tx
			.insert(models)
			.values({
				provider: "REPLICATE",
				name: "tmappdev/lang-segment-anything",
				version: langSegmentAnythingVersion,
				type: "segmentation",
				inputSchema: langSegmentationInputSchema,
				outputSchema: langSegmentationOutputSchema,
				config: langSegmentationModelConfig,
			})
			.returning({ id: models.id });
		if (!langSegmentationModel) {
			throw new Error("Failed to create language segmentation model");
		}

		// ── Tools ──

		const [createDocDescTool] = await tx
			.insert(tools)
			.values({
				name: "create_document_description",
				description: "Save an LLM-generated text description for a document.",
				inputSchema: {
					type: "object",
					properties: {
						document_id: { type: "string" },
						text: { type: "string" },
						model_provider: { type: "string" },
						model_name: { type: "string" },
						model_version: { type: "string" },
					},
					required: [
						"document_id",
						"text",
						"model_provider",
						"model_name",
						"model_version",
					],
				},
				outputSchema: {
					type: "object",
					properties: {
						description_id: { type: "string" },
						text: { type: "string" },
					},
				},
			})
			.returning({ id: tools.id });
		if (!createDocDescTool)
			throw new Error("Failed to create create_document_description tool");

		const [createDocEmbTool] = await tx
			.insert(tools)
			.values({
				name: "create_document_embedding",
				description:
					"Generate a CLIP embedding for a document image and save it to the database.",
				inputSchema: {
					type: "object",
					properties: {
						document_id: { type: "string" },
						temp_url: { type: "string" },
					},
					required: ["document_id", "temp_url"],
				},
				outputSchema: {
					type: "object",
					properties: {
						embedding_id: { type: "string" },
					},
				},
			})
			.returning({ id: tools.id });
		if (!createDocEmbTool)
			throw new Error("Failed to create create_document_embedding tool");

		const [createDocSegTool] = await tx
			.insert(tools)
			.values({
				name: "create_document_segmentation",
				description:
					"Generate a document segmentation, persist the result payload, and store any derived segmented image as a document.",
				inputSchema: {
					type: "object",
					properties: {
						document_id: { type: "string" },
						temp_url: { type: "string" },
						model_provider: { type: "string" },
						model_name: { type: "string" },
						model_version: { type: "string" },
						input_params: { type: "object" },
					},
					required: [
						"document_id",
						"temp_url",
						"model_provider",
						"model_name",
						"model_version",
					],
				},
				outputSchema: {
					type: "object",
					properties: {
						segmentation_id: { type: "string" },
						segmented_document_id: {
							type: ["string", "null"],
						},
						segmented_temp_url: {
							type: ["string", "null"],
						},
						result: {},
					},
				},
			})
			.returning({ id: tools.id });
		if (!createDocSegTool)
			throw new Error("Failed to create create_document_segmentation tool");

		const [createDescEmbTool] = await tx
			.insert(tools)
			.values({
				name: "create_description_embedding",
				description:
					"Generate a CLIP text embedding for a document description and save it to the database.",
				inputSchema: {
					type: "object",
					properties: {
						document_description_id: { type: "string" },
						text: { type: "string" },
					},
					required: ["document_description_id", "text"],
				},
				outputSchema: {
					type: "object",
					properties: {
						embedding_id: { type: "string" },
					},
				},
			})
			.returning({ id: tools.id });
		if (!createDescEmbTool)
			throw new Error("Failed to create create_description_embedding tool");

		const [findSimilarDocsTool] = await tx
			.insert(tools)
			.values({
				name: "find_similar_documents",
				description:
					"Find documents with similar CLIP embeddings using cosine distance.",
				inputSchema: {
					type: "object",
					properties: {
						document_id: { type: "string" },
					},
					required: ["document_id"],
				},
				outputSchema: {
					type: "object",
					properties: {
						matches: {
							type: "array",
							items: {
								type: "object",
								properties: {
									id: { type: "string" },
									distance: { type: "number" },
								},
							},
						},
					},
				},
			})
			.returning({ id: tools.id });
		if (!findSimilarDocsTool)
			throw new Error("Failed to create find_similar_documents tool");

		const [findSimilarDescsTool] = await tx
			.insert(tools)
			.values({
				name: "find_similar_descriptions",
				description:
					"Find document descriptions with similar CLIP embeddings using cosine distance.",
				inputSchema: {
					type: "object",
					properties: {
						document_description_id: { type: "string" },
					},
					required: ["document_description_id"],
				},
				outputSchema: {
					type: "object",
					properties: {
						matches: {
							type: "array",
							items: {
								type: "object",
								properties: {
									id: { type: "string" },
									distance: { type: "number" },
								},
							},
						},
					},
				},
			})
			.returning({ id: tools.id });
		if (!findSimilarDescsTool)
			throw new Error("Failed to create find_similar_descriptions tool");

		// ── Agent Graph (workflow 1: describe → save → embed → find similar) ──

		const [pipelineGraph] = await tx
			.insert(agentGraphs)
			.values({
				name: "Document Processing Pipeline",
				description:
					"Describes a document image, saves the description, embeds both, and finds similar items.",
				entryNode: "describe",
				organizationId: organization.id,
			})
			.returning({ id: agentGraphs.id });
		if (!pipelineGraph)
			throw new Error("Failed to create pipeline agent graph");

		// Node: describe (worker) — LLM describes the image
		const [describeNode] = await tx
			.insert(agentGraphNodes)
			.values({
				nodeKey: "describe",
				nodeType: "worker",
				inputKey: "temp_url",
				outputKey: "description",
				config: {
					system_message:
						"You are a document analysis assistant. Given an image URL of a document, return one concise plain-text paragraph describing key contents, layout, and any visible text. Keep the output to 50 words max (about 320 characters), and do not include markdown, bullet points, or URLs.",
					max_iterations: 3,
					input_mode: "image_url",
					input_prompt:
						"Describe this document image in one concise paragraph (max 50 words). Focus on layout, key text, and visual elements.",
				},
				agentGraphId: pipelineGraph.id,
				modelId: gpt41MiniModel.id,
			})
			.returning({ id: agentGraphNodes.id });
		if (!describeNode) throw new Error("Failed to create describe node");

		// Node: save_description (tool) — saves description to DB
		const [saveDescNode] = await tx
			.insert(agentGraphNodes)
			.values({
				nodeKey: "save_description",
				nodeType: "tool",
				config: {
					input_mapping: {
						text: "description",
						model_provider: "_const:OPENAI",
						model_name: "_const:gpt-4.1-mini",
						model_version: "_const:",
					},
					output_mapping: {
						description_id: "document_description_id",
					},
				},
				agentGraphId: pipelineGraph.id,
			})
			.returning({ id: agentGraphNodes.id });
		if (!saveDescNode)
			throw new Error("Failed to create save_description node");

		// Node: embed_document (tool) — CLIP embed the document image
		const [embedDocNode] = await tx
			.insert(agentGraphNodes)
			.values({
				nodeKey: "embed_document",
				nodeType: "tool",
				config: {},
				agentGraphId: pipelineGraph.id,
			})
			.returning({ id: agentGraphNodes.id });
		if (!embedDocNode) throw new Error("Failed to create embed_document node");

		// Node: embed_description (tool) — CLIP embed the description text
		const [embedDescNode] = await tx
			.insert(agentGraphNodes)
			.values({
				nodeKey: "embed_description",
				nodeType: "tool",
				config: {
					input_mapping: {
						text: "description",
						document_description_id: "document_description_id",
					},
				},
				agentGraphId: pipelineGraph.id,
			})
			.returning({ id: agentGraphNodes.id });
		if (!embedDescNode)
			throw new Error("Failed to create embed_description node");

		// Node: find_similar_docs (tool)
		const [findDocsNode] = await tx
			.insert(agentGraphNodes)
			.values({
				nodeKey: "find_similar_docs",
				nodeType: "tool",
				config: {},
				agentGraphId: pipelineGraph.id,
			})
			.returning({ id: agentGraphNodes.id });
		if (!findDocsNode)
			throw new Error("Failed to create find_similar_docs node");

		// Node: find_similar_descs (tool)
		const [findDescsNode] = await tx
			.insert(agentGraphNodes)
			.values({
				nodeKey: "find_similar_descs",
				nodeType: "tool",
				config: {},
				agentGraphId: pipelineGraph.id,
			})
			.returning({ id: agentGraphNodes.id });
		if (!findDescsNode)
			throw new Error("Failed to create find_similar_descs node");

		// ── Node-Tool associations ──

		await tx.insert(agentGraphNodeTools).values([
			{
				agentGraphNodeId: saveDescNode.id,
				toolId: createDocDescTool.id,
			},
			{
				agentGraphNodeId: embedDocNode.id,
				toolId: createDocEmbTool.id,
			},
			{
				agentGraphNodeId: embedDescNode.id,
				toolId: createDescEmbTool.id,
			},
			{
				agentGraphNodeId: findDocsNode.id,
				toolId: findSimilarDocsTool.id,
			},
			{
				agentGraphNodeId: findDescsNode.id,
				toolId: findSimilarDescsTool.id,
			},
		]);

		// ── Edges (linear pipeline) ──

		await tx.insert(agentGraphEdges).values([
			{
				fromNode: "describe",
				toNode: "save_description",
				agentGraphId: pipelineGraph.id,
			},
			{
				fromNode: "save_description",
				toNode: "embed_document",
				agentGraphId: pipelineGraph.id,
			},
			{
				fromNode: "embed_document",
				toNode: "embed_description",
				agentGraphId: pipelineGraph.id,
			},
			{
				fromNode: "embed_description",
				toNode: "find_similar_docs",
				agentGraphId: pipelineGraph.id,
			},
			{
				fromNode: "find_similar_docs",
				toNode: "find_similar_descs",
				agentGraphId: pipelineGraph.id,
			},
			{
				fromNode: "find_similar_descs",
				toNode: "END",
				agentGraphId: pipelineGraph.id,
			},
		]);

		// ── Device (linked to workflow 1 graph) ──

		const [pipelineDevice] = await tx
			.insert(devices)
			.values({
				name: "Seed Device",
				slug: "seed-device",
				organizationId: organization.id,
				projectId: project.id,
				agentGraphId: pipelineGraph.id,
			})
			.returning({
				id: devices.id,
				slug: devices.slug,
			});
		if (!pipelineDevice)
			throw new Error("Failed to create seed pipeline device");

		// ── API Key (workflow 1 device) ──

		const [pipelineApiKey] = await tx
			.insert(apikeys)
			.values({
				name: "Seed Device API Key",
				start: plainPipelineApiKey.slice(0, 6),
				prefix: "seed",
				key: hashedPipelineApiKey,
				userId: user.id,
				organizationId: organization.id,
				projectId: project.id,
				deviceId: pipelineDevice.id,
				enabled: true,
				rateLimitEnabled: true,
				rateLimitTimeWindow: 86_400_000,
				rateLimitMax: 10,
				requestCount: 0,
				createdAt: now,
				updatedAt: now,
				permissions: JSON.stringify({ uploads: ["presign", "ack"] }),
				metadata: JSON.stringify({ source: "seed.ts" }),
			})
			.returning({
				id: apikeys.id,
			});
		if (!pipelineApiKey)
			throw new Error("Failed to create seed pipeline API key");

		// ── Seed documents, descriptions, and embeddings ──

		const insertedDocuments = await tx
			.insert(documents)
			.values(
				uploadedSeedDocuments.map((seedDocument) => ({
					bucket,
					objectKey: seedDocument.objectKey,
					contentType: seedDocument.contentType,
					eTag: seedDocument.eTag,
					sizeBytes: seedDocument.sizeBytes,
					lastModifiedAt: seedDocument.lastModifiedAt,
					visibility: "org",
					organizationId: organization.id,
					projectId: project.id,
					deviceId: pipelineDevice.id,
				})),
			)
			.returning({
				id: documents.id,
				objectKey: documents.objectKey,
			});

		if (insertedDocuments.length !== uploadedSeedDocuments.length) {
			throw new Error("Failed to insert all seed documents");
		}

		const documentIdByObjectKey = new Map(
			insertedDocuments.map((document) => [document.objectKey, document.id]),
		);

		const seedDocumentsWithIds = uploadedSeedDocuments.map((seedDocument) => {
			const documentId = documentIdByObjectKey.get(seedDocument.objectKey);
			if (!documentId) {
				throw new Error(
					`Failed to resolve document id for ${seedDocument.objectKey}`,
				);
			}

			return {
				...seedDocument,
				documentId,
			};
		});

		const insertedDescriptions = await tx
			.insert(documentDescriptions)
			.values(
				seedDocumentsWithIds.map((seedDocument) => ({
					documentId: seedDocument.documentId,
					modelId: gpt41MiniModel.id,
					text: seedDocument.description,
				})),
			)
			.returning({
				id: documentDescriptions.id,
				documentId: documentDescriptions.documentId,
				text: documentDescriptions.text,
			});

		if (insertedDescriptions.length !== seedDocumentsWithIds.length) {
			throw new Error("Failed to insert all seed document descriptions");
		}

		await tx.insert(documentEmbeddings).values(
			seedDocumentsWithIds.map((seedDocument) => ({
				documentId: seedDocument.documentId,
				modelId: clipModel.id,
				embeddingDim: seedDocumentEmbeddingDim,
				embedding: createDeterministicEmbedding(
					`document:${seedDocument.objectKey}`,
				),
			})),
		);

		await tx.insert(documentDescriptionEmbeddings).values(
			insertedDescriptions.map((description) => ({
				documentDescriptionId: description.id,
				modelId: clipModel.id,
				embeddingDim: seedDocumentEmbeddingDim,
				embedding: createDeterministicEmbedding(
					`description:${description.documentId}:${description.text}`,
				),
			})),
		);

		// ── Agent Graph (workflow 2: supervisor routes to good vs bad image-quality workers) ──

		const [qualityReviewGraph] = await tx
			.insert(agentGraphs)
			.values({
				name: "Image Quality Review",
				description:
					"Routes an uploaded image to either a good-quality reviewer or a bad-quality reviewer, then returns a detailed justification.",
				entryNode: "quality_review_supervisor",
				organizationId: organization.id,
			})
			.returning({ id: agentGraphs.id });
		if (!qualityReviewGraph)
			throw new Error("Failed to create quality review agent graph");

		const [goodImageQualityWorkerNode] = await tx
			.insert(agentGraphNodes)
			.values({
				nodeKey: "image_quality_good_worker",
				nodeType: "worker",
				config: {
					system_message:
						"You are the GOOD image-quality specialist. Use this route only when the image quality is acceptable for downstream document analysis. Given an image URL, explain in detail why the image is good enough. Cover sharpness/focus, exposure and contrast, framing/cropping, legibility of text, noise/compression artifacts, and whether the full document content appears captured. Mention minor flaws if present, but keep the final judgment clearly positive. End with a one-line verdict that starts with 'Verdict: GOOD'.",
					max_iterations: 4,
				},
				agentGraphId: qualityReviewGraph.id,
				modelId: gpt41MiniModel.id,
			})
			.returning({ id: agentGraphNodes.id });
		if (!goodImageQualityWorkerNode)
			throw new Error("Failed to create image_quality_good_worker node");

		const [badImageQualityWorkerNode] = await tx
			.insert(agentGraphNodes)
			.values({
				nodeKey: "image_quality_bad_worker",
				nodeType: "worker",
				config: {
					system_message:
						"You are the BAD image-quality specialist. Use this route only when the image quality is not acceptable for downstream document analysis. Given an image URL, explain in detail why the image is bad. Be specific about failure points such as blur, motion blur, poor exposure, glare, cropping, skew/perspective distortion, low resolution, illegible text, and compression artifacts. Include concrete remediation steps so the user can retake the image successfully. End with a one-line verdict that starts with 'Verdict: BAD'.",
					max_iterations: 4,
				},
				agentGraphId: qualityReviewGraph.id,
				modelId: gpt41MiniModel.id,
			})
			.returning({ id: agentGraphNodes.id });
		if (!badImageQualityWorkerNode)
			throw new Error("Failed to create image_quality_bad_worker node");

		const [qualityReviewSupervisorNode] = await tx
			.insert(agentGraphNodes)
			.values({
				nodeKey: "quality_review_supervisor",
				nodeType: "supervisor",
				inputKey: "temp_url",
				outputKey: "quality_review",
				config: {
					members: ["image_quality_good_worker", "image_quality_bad_worker"],
					input_mode: "image_url",
					input_prompt:
						"Route this image to exactly one specialist based on quality. After that specialist responds, finish.",
					max_iterations: 10,
				},
				agentGraphId: qualityReviewGraph.id,
				modelId: gpt41MiniModel.id,
			})
			.returning({ id: agentGraphNodes.id });
		if (!qualityReviewSupervisorNode)
			throw new Error("Failed to create quality_review_supervisor node");

		// No edges needed for supervisor graphs — BuildGraph auto-wires:
		// - supervisor -> members (conditional edge from routing decision)
		// - members -> supervisor (static edges back)
		// - supervisor -> END (conditional edge when FINISH)

		// ── Device + API key (workflow 2 graph) ──

		const [qualityReviewDevice] = await tx
			.insert(devices)
			.values({
				name: "Seed Quality Review Device",
				slug: "seed-quality-review-device",
				organizationId: organization.id,
				projectId: project.id,
				agentGraphId: qualityReviewGraph.id,
			})
			.returning({
				id: devices.id,
				slug: devices.slug,
			});
		if (!qualityReviewDevice)
			throw new Error("Failed to create seed quality review device");

		const [qualityReviewApiKey] = await tx
			.insert(apikeys)
			.values({
				name: "Seed Quality Review Device API Key",
				start: plainQualityReviewApiKey.slice(0, 6),
				prefix: "seed",
				key: hashedQualityReviewApiKey,
				userId: user.id,
				organizationId: organization.id,
				projectId: project.id,
				deviceId: qualityReviewDevice.id,
				enabled: true,
				rateLimitEnabled: true,
				rateLimitTimeWindow: 86_400_000,
				rateLimitMax: 10,
				requestCount: 0,
				createdAt: now,
				updatedAt: now,
				permissions: JSON.stringify({ uploads: ["presign", "ack"] }),
				metadata: JSON.stringify({
					source: "seed.ts",
					workflow: "quality-review",
				}),
			})
			.returning({
				id: apikeys.id,
			});
		if (!qualityReviewApiKey)
			throw new Error("Failed to create seed quality review API key");

		// ── Seed documents for quality review device ──

		const insertedQRDocuments = await tx
			.insert(documents)
			.values(
				uploadedQualityReviewDocuments.map((seedDocument) => ({
					bucket,
					objectKey: seedDocument.objectKey,
					contentType: seedDocument.contentType,
					eTag: seedDocument.eTag,
					sizeBytes: seedDocument.sizeBytes,
					lastModifiedAt: seedDocument.lastModifiedAt,
					visibility: "org",
					organizationId: organization.id,
					projectId: project.id,
					deviceId: qualityReviewDevice.id,
				})),
			)
			.returning({
				id: documents.id,
				objectKey: documents.objectKey,
			});

		if (insertedQRDocuments.length !== uploadedQualityReviewDocuments.length) {
			throw new Error("Failed to insert all quality review seed documents");
		}

		const qrDocumentIdByObjectKey = new Map(
			insertedQRDocuments.map((document) => [document.objectKey, document.id]),
		);

		const qrSeedDocumentsWithIds = uploadedQualityReviewDocuments.map(
			(seedDocument) => {
				const documentId = qrDocumentIdByObjectKey.get(seedDocument.objectKey);
				if (!documentId) {
					throw new Error(
						`Failed to resolve document id for ${seedDocument.objectKey}`,
					);
				}

				return {
					...seedDocument,
					documentId,
				};
			},
		);

		const insertedQRDescriptions = await tx
			.insert(documentDescriptions)
			.values(
				qrSeedDocumentsWithIds.map((seedDocument) => ({
					documentId: seedDocument.documentId,
					modelId: gpt41MiniModel.id,
					text: seedDocument.description,
				})),
			)
			.returning({
				id: documentDescriptions.id,
				documentId: documentDescriptions.documentId,
				text: documentDescriptions.text,
			});

		if (insertedQRDescriptions.length !== qrSeedDocumentsWithIds.length) {
			throw new Error(
				"Failed to insert all quality review seed document descriptions",
			);
		}

		await tx.insert(documentEmbeddings).values(
			qrSeedDocumentsWithIds.map((seedDocument) => ({
				documentId: seedDocument.documentId,
				modelId: clipModel.id,
				embeddingDim: seedDocumentEmbeddingDim,
				embedding: createDeterministicEmbedding(
					`document:${seedDocument.objectKey}`,
				),
			})),
		);

		await tx.insert(documentDescriptionEmbeddings).values(
			insertedQRDescriptions.map((description) => ({
				documentDescriptionId: description.id,
				modelId: clipModel.id,
				embeddingDim: seedDocumentEmbeddingDim,
				embedding: createDeterministicEmbedding(
					`description:${description.documentId}:${description.text}`,
				),
			})),
		);

		// ── Example Agent Graph Runs (quality review supervisor workflow) ──

		const runBaseTime = new Date(now.getTime() - 1000 * 60 * 10); // 10 min ago

		// Run 1: sharp-puppy → routed to GOOD worker (completed)
		const goodRunStarted = new Date(runBaseTime.getTime());
		const goodRunFinished = new Date(runBaseTime.getTime() + 1000 * 12); // 12s

		const sharpPuppyDoc = qrSeedDocumentsWithIds.find(
			(d) => d.slug === "sharp-puppy",
		);
		if (!sharpPuppyDoc)
			throw new Error("Failed to find sharp-puppy seed document");

		const [goodRun] = await tx
			.insert(agentGraphRuns)
			.values({
				agentGraphId: qualityReviewGraph.id,
				status: "completed",
				initialState: {
					temp_url: `https://s3.example.com/${sharpPuppyDoc.objectKey}`,
					document_id: sharpPuppyDoc.documentId,
				},
				finalState: {
					temp_url: `https://s3.example.com/${sharpPuppyDoc.objectKey}`,
					document_id: sharpPuppyDoc.documentId,
					quality_review:
						"The puppy portrait is sharp and evenly exposed with crisp fur detail around the face and clear catchlights in the eyes. Subject separation is strong, framing is intentional, and the wooden background adds texture without distracting from the subject. Minor vignette styling is present but does not reduce readability of the main subject. Verdict: GOOD",
					routed_to: "image_quality_good_worker",
				},
				startedAt: goodRunStarted,
				finishedAt: goodRunFinished,
			})
			.returning({ id: agentGraphRuns.id });
		if (!goodRun) throw new Error("Failed to create good quality run");

		await tx.insert(agentGraphRunSteps).values([
			{
				runId: goodRun.id,
				nodeKey: "quality_review_supervisor",
				stepOrder: 1,
				stateDelta: {
					routing_decision: "image_quality_good_worker",
					reasoning:
						"Image is sharp, well-exposed, and clearly framed. Routing to good-quality specialist.",
				},
				startedAt: new Date(goodRunStarted.getTime()),
				finishedAt: new Date(goodRunStarted.getTime() + 1000 * 3),
			},
			{
				runId: goodRun.id,
				nodeKey: "image_quality_good_worker",
				stepOrder: 2,
				stateDelta: {
					quality_review:
						"The puppy portrait is sharp and evenly exposed with crisp fur detail around the face and clear catchlights in the eyes. Subject separation is strong, framing is intentional, and the wooden background adds texture without distracting from the subject. Minor vignette styling is present but does not reduce readability of the main subject. Verdict: GOOD",
				},
				startedAt: new Date(goodRunStarted.getTime() + 1000 * 3),
				finishedAt: new Date(goodRunStarted.getTime() + 1000 * 9),
			},
			{
				runId: goodRun.id,
				nodeKey: "quality_review_supervisor",
				stepOrder: 3,
				stateDelta: {
					decision: "FINISH",
					routed_to: "image_quality_good_worker",
				},
				startedAt: new Date(goodRunStarted.getTime() + 1000 * 9),
				finishedAt: new Date(goodRunStarted.getTime() + 1000 * 12),
			},
		]);

		// Run 2: soft-focus-cat → routed to BAD worker (completed)
		const badRunStarted = new Date(runBaseTime.getTime() + 1000 * 30);
		const badRunFinished = new Date(badRunStarted.getTime() + 1000 * 14);

		const softFocusCatDoc = qrSeedDocumentsWithIds.find(
			(d) => d.slug === "soft-focus-cat",
		);
		if (!softFocusCatDoc)
			throw new Error("Failed to find soft-focus-cat seed document");

		const [badRun] = await tx
			.insert(agentGraphRuns)
			.values({
				agentGraphId: qualityReviewGraph.id,
				status: "completed",
				initialState: {
					temp_url: `https://s3.example.com/${softFocusCatDoc.objectKey}`,
					document_id: softFocusCatDoc.documentId,
				},
				finalState: {
					temp_url: `https://s3.example.com/${softFocusCatDoc.objectKey}`,
					document_id: softFocusCatDoc.documentId,
					quality_review:
						"The cat close-up uses such a shallow depth of field that only the nose is sharply rendered while the rest of the frame falls off into blur. That makes the subject poorly documented for review or downstream extraction tasks. Remediation: step back slightly, increase depth of field, and ensure the full face stays in focus before capture. Verdict: BAD",
					routed_to: "image_quality_bad_worker",
				},
				startedAt: badRunStarted,
				finishedAt: badRunFinished,
			})
			.returning({ id: agentGraphRuns.id });
		if (!badRun) throw new Error("Failed to create bad quality run");

		await tx.insert(agentGraphRunSteps).values([
			{
				runId: badRun.id,
				nodeKey: "quality_review_supervisor",
				stepOrder: 1,
				stateDelta: {
					routing_decision: "image_quality_bad_worker",
					reasoning:
						"Image has severe focus falloff and poor overall subject coverage. Routing to bad-quality specialist.",
				},
				startedAt: new Date(badRunStarted.getTime()),
				finishedAt: new Date(badRunStarted.getTime() + 1000 * 3),
			},
			{
				runId: badRun.id,
				nodeKey: "image_quality_bad_worker",
				stepOrder: 2,
				stateDelta: {
					quality_review:
						"The cat close-up uses such a shallow depth of field that only the nose is sharply rendered while the rest of the frame falls off into blur. That makes the subject poorly documented for review or downstream extraction tasks. Remediation: step back slightly, increase depth of field, and ensure the full face stays in focus before capture. Verdict: BAD",
				},
				startedAt: new Date(badRunStarted.getTime() + 1000 * 3),
				finishedAt: new Date(badRunStarted.getTime() + 1000 * 11),
			},
			{
				runId: badRun.id,
				nodeKey: "quality_review_supervisor",
				stepOrder: 3,
				stateDelta: {
					decision: "FINISH",
					routed_to: "image_quality_bad_worker",
				},
				startedAt: new Date(badRunStarted.getTime() + 1000 * 11),
				finishedAt: new Date(badRunStarted.getTime() + 1000 * 14),
			},
		]);

		// Run 3: washed-out-shoreline → routed to BAD worker (completed)
		const washedOutRunStarted = new Date(runBaseTime.getTime() + 1000 * 60);
		const washedOutRunFinished = new Date(
			washedOutRunStarted.getTime() + 1000 * 15,
		);

		const washedOutShorelineDoc = qrSeedDocumentsWithIds.find(
			(d) => d.slug === "washed-out-shoreline",
		);
		if (!washedOutShorelineDoc)
			throw new Error("Failed to find washed-out-shoreline seed document");

		const [washedOutRun] = await tx
			.insert(agentGraphRuns)
			.values({
				agentGraphId: qualityReviewGraph.id,
				status: "completed",
				initialState: {
					temp_url: `https://s3.example.com/${washedOutShorelineDoc.objectKey}`,
					document_id: washedOutShorelineDoc.documentId,
				},
				finalState: {
					temp_url: `https://s3.example.com/${washedOutShorelineDoc.objectKey}`,
					document_id: washedOutShorelineDoc.documentId,
					quality_review:
						"The shoreline image is washed out by haze and flat lighting, leaving weak separation between the rocks, water, and sky. Fine detail is muted and the overall scene lacks the contrast needed for reliable visual inspection. Remediation: retake in clearer light, reduce overexposure, and increase local contrast so subject boundaries remain distinct. Verdict: BAD",
					routed_to: "image_quality_bad_worker",
				},
				startedAt: washedOutRunStarted,
				finishedAt: washedOutRunFinished,
			})
			.returning({ id: agentGraphRuns.id });
		if (!washedOutRun)
			throw new Error("Failed to create washed-out shoreline run");

		await tx.insert(agentGraphRunSteps).values([
			{
				runId: washedOutRun.id,
				nodeKey: "quality_review_supervisor",
				stepOrder: 1,
				stateDelta: {
					routing_decision: "image_quality_bad_worker",
					reasoning:
						"Image has low contrast, muted detail, and weak subject separation. Routing to bad-quality specialist.",
				},
				startedAt: new Date(washedOutRunStarted.getTime()),
				finishedAt: new Date(washedOutRunStarted.getTime() + 1000 * 4),
			},
			{
				runId: washedOutRun.id,
				nodeKey: "image_quality_bad_worker",
				stepOrder: 2,
				stateDelta: {
					quality_review:
						"The shoreline image is washed out by haze and flat lighting, leaving weak separation between the rocks, water, and sky. Fine detail is muted and the overall scene lacks the contrast needed for reliable visual inspection. Remediation: retake in clearer light, reduce overexposure, and increase local contrast so subject boundaries remain distinct. Verdict: BAD",
				},
				startedAt: new Date(washedOutRunStarted.getTime() + 1000 * 4),
				finishedAt: new Date(washedOutRunStarted.getTime() + 1000 * 12),
			},
			{
				runId: washedOutRun.id,
				nodeKey: "quality_review_supervisor",
				stepOrder: 3,
				stateDelta: {
					decision: "FINISH",
					routed_to: "image_quality_bad_worker",
				},
				startedAt: new Date(washedOutRunStarted.getTime() + 1000 * 12),
				finishedAt: new Date(washedOutRunStarted.getTime() + 1000 * 15),
			},
		]);

		// ── Agent Graph (workflow 3: semantic segmentation + language-prompted segmentation) ──

		const [segmentationGraph] = await tx
			.insert(agentGraphs)
			.values({
				name: "Document Segmentation Showcase",
				description:
					"Generates a short prompt from the image, runs prompt-based segmentation, and summarizes the derived output.",
				entryNode: "suggest_segment_prompt",
				organizationId: organization.id,
			})
			.returning({ id: agentGraphs.id });
		if (!segmentationGraph) {
			throw new Error("Failed to create segmentation agent graph");
		}

		const [suggestSegmentPromptNode] = await tx
			.insert(agentGraphNodes)
			.values({
				nodeKey: "suggest_segment_prompt",
				nodeType: "worker",
				inputKey: "temp_url",
				outputKey: "segmentation_prompt",
				config: {
					system_message:
						"You create short segmentation prompts from images. Return only the most important visible subject as a concise noun phrase of one to four words. Do not add punctuation, explanations, or multiple options.",
					max_iterations: 2,
					input_mode: "image_url",
					input_prompt:
						"Look at this image and return the single best subject to segment as a short noun phrase.",
				},
				agentGraphId: segmentationGraph.id,
				modelId: gpt41MiniModel.id,
			})
			.returning({ id: agentGraphNodes.id });
		if (!suggestSegmentPromptNode) {
			throw new Error("Failed to create suggest_segment_prompt node");
		}

		const [langSegmentNode] = await tx
			.insert(agentGraphNodes)
			.values({
				nodeKey: "lang_segment",
				nodeType: "tool",
				config: {
					input_mapping: {
						model_provider: "_const:REPLICATE",
						model_name: "_const:tmappdev/lang-segment-anything",
						model_version: `_const:${langSegmentAnythingVersion}`,
						input_params: {
							text_prompt: "segmentation_prompt",
						},
					},
					output_mapping: {
						segmentation_id: "lang_segmentation_id",
						segmented_document_id: "lang_segmented_document_id",
						segmented_temp_url: "lang_segmented_temp_url",
						result: "lang_segmentation_result",
					},
				},
				agentGraphId: segmentationGraph.id,
			})
			.returning({ id: agentGraphNodes.id });
		if (!langSegmentNode) {
			throw new Error("Failed to create lang_segment node");
		}

		const [describeLangNode] = await tx
			.insert(agentGraphNodes)
			.values({
				nodeKey: "describe_lang_segment",
				nodeType: "worker",
				inputKey: "lang_segmented_temp_url",
				outputKey: "lang_segment_summary",
				config: {
					system_message:
						"You are a segmentation analyst. Given a prompt-driven segmented image, explain what object the mask appears to isolate and whether the result is plausible. Keep the response to one concise paragraph.",
					max_iterations: 3,
					input_mode: "image_url",
					input_prompt:
						"Describe the prompt-driven segmentation result and the object it isolates.",
				},
				agentGraphId: segmentationGraph.id,
				modelId: gpt41MiniModel.id,
			})
			.returning({ id: agentGraphNodes.id });
		if (!describeLangNode) {
			throw new Error("Failed to create describe_lang_segment node");
		}

		await tx.insert(agentGraphNodeTools).values([
			{
				agentGraphNodeId: langSegmentNode.id,
				toolId: createDocSegTool.id,
			},
		]);

		await tx.insert(agentGraphEdges).values([
			{
				fromNode: "suggest_segment_prompt",
				toNode: "lang_segment",
				agentGraphId: segmentationGraph.id,
			},
			{
				fromNode: "lang_segment",
				toNode: "describe_lang_segment",
				agentGraphId: segmentationGraph.id,
			},
			{
				fromNode: "describe_lang_segment",
				toNode: "END",
				agentGraphId: segmentationGraph.id,
			},
		]);

		const [segmentationDevice] = await tx
			.insert(devices)
			.values({
				name: "Seed Segmentation Device",
				slug: "seed-segmentation-device",
				organizationId: organization.id,
				projectId: project.id,
				agentGraphId: segmentationGraph.id,
			})
			.returning({
				id: devices.id,
				slug: devices.slug,
			});
		if (!segmentationDevice) {
			throw new Error("Failed to create segmentation device");
		}

		const [segmentationApiKey] = await tx
			.insert(apikeys)
			.values({
				name: "Seed Segmentation Device API Key",
				start: plainSegmentationApiKey.slice(0, 6),
				prefix: "seed",
				key: hashedSegmentationApiKey,
				userId: user.id,
				organizationId: organization.id,
				projectId: project.id,
				deviceId: segmentationDevice.id,
				enabled: true,
				rateLimitEnabled: true,
				rateLimitTimeWindow: 86_400_000,
				rateLimitMax: 10,
				requestCount: 0,
				createdAt: now,
				updatedAt: now,
				permissions: JSON.stringify({ uploads: ["presign", "ack"] }),
				metadata: JSON.stringify({
					source: "seed.ts",
					workflow: "segmentation-showcase",
				}),
			})
			.returning({
				id: apikeys.id,
			});
		if (!segmentationApiKey) {
			throw new Error("Failed to create segmentation API key");
		}

		const insertedSegmentationDocuments = await tx
			.insert(documents)
			.values(
				uploadedSegmentationDocuments.map((seedDocument) => ({
					bucket,
					objectKey: seedDocument.objectKey,
					contentType: seedDocument.contentType,
					eTag: seedDocument.eTag,
					sizeBytes: seedDocument.sizeBytes,
					lastModifiedAt: seedDocument.lastModifiedAt,
					visibility: "org",
					organizationId: organization.id,
					projectId: project.id,
					deviceId: segmentationDevice.id,
				})),
			)
			.returning({
				id: documents.id,
				objectKey: documents.objectKey,
			});
		if (
			insertedSegmentationDocuments.length !==
			uploadedSegmentationDocuments.length
		) {
			throw new Error("Failed to insert segmentation seed documents");
		}

		const segmentationDocumentIdByObjectKey = new Map(
			insertedSegmentationDocuments.map((document) => [
				document.objectKey,
				document.id,
			]),
		);

		const segmentationSeedDocumentsWithIds = uploadedSegmentationDocuments.map(
			(seedDocument) => {
				const documentId = segmentationDocumentIdByObjectKey.get(
					seedDocument.objectKey,
				);
				if (!documentId) {
					throw new Error(
						`Failed to resolve segmentation document id for ${seedDocument.objectKey}`,
					);
				}

				return {
					...seedDocument,
					documentId,
				};
			},
		);

		return {
			user,
			dashboardSession,
			organization,
			project,
			pipelineDevice,
			pipelineApiKey,
			pipelineGraph,
			qualityReviewDevice,
			qualityReviewApiKey,
			qualityReviewGraph,
			segmentationDevice,
			segmentationApiKey,
			segmentationGraph,
			clipModel,
			gpt41MiniModel,
			semanticSegmentationModel,
			langSegmentationModel,
			seededDocuments: seedDocumentsWithIds,
			seededDescriptionCount: insertedDescriptions.length,
			seededQRDocuments: qrSeedDocumentsWithIds,
			seededQRDescriptionCount: insertedQRDescriptions.length,
			seededSegmentationDocuments: segmentationSeedDocumentsWithIds,
			agentRuns: {
				goodRun: {
					id: goodRun.id,
					document: "sharp-puppy",
					verdict: "GOOD",
				},
				badRun: { id: badRun.id, document: "soft-focus-cat", verdict: "BAD" },
				washedOutRun: {
					id: washedOutRun.id,
					document: "washed-out-shoreline",
					verdict: "BAD",
				},
			},
		};
	});

	console.log("\nSeed complete\n");
	console.log(`User: ${result.user.email} (${result.user.id})`);
	console.log(`Session ID: ${result.dashboardSession.id}`);
	console.log(`Session Token (dashboard): ${result.dashboardSession.token}`);
	console.log(
		`Session Expires At: ${result.dashboardSession.expiresAt.toISOString()}`,
	);
	console.log(
		"Use cookie `better-auth.session_token` with the session token for local dashboard auth.",
	);
	console.log(
		`Organization: ${result.organization.slug} (${result.organization.id})`,
	);
	console.log(`Project: ${result.project.slug} (${result.project.id})`);
	console.log(
		`Device (workflow 1): ${result.pipelineDevice.slug} (${result.pipelineDevice.id})`,
	);
	console.log(`API Key ID (workflow 1): ${result.pipelineApiKey.id}`);
	console.log(`API Key (plain, workflow 1): ${plainPipelineApiKey}`);
	console.log(
		`Device (workflow 2): ${result.qualityReviewDevice.slug} (${result.qualityReviewDevice.id})`,
	);
	console.log(`API Key ID (workflow 2): ${result.qualityReviewApiKey.id}`);
	console.log(`API Key (plain, workflow 2): ${plainQualityReviewApiKey}`);
	console.log(
		`Device (workflow 3): ${result.segmentationDevice.slug} (${result.segmentationDevice.id})`,
	);
	console.log(`API Key ID (workflow 3): ${result.segmentationApiKey.id}`);
	console.log(`API Key (plain, workflow 3): ${plainSegmentationApiKey}`);
	console.log(`\nAgent Graph (workflow 1): ${result.pipelineGraph.id}`);
	console.log(`Agent Graph (workflow 2): ${result.qualityReviewGraph.id}`);
	console.log(`Agent Graph (workflow 3): ${result.segmentationGraph.id}`);
	console.log(`CLIP Model: ${result.clipModel.id}`);
	console.log(`GPT-4.1 Mini Model: ${result.gpt41MiniModel.id}`);
	console.log(
		`Semantic segmentation model: ${result.semanticSegmentationModel.id}`,
	);
	console.log(
		`Language segmentation model: ${result.langSegmentationModel.id}`,
	);
	console.log(`Seeded documents (pipeline): ${result.seededDocuments.length}`);
	console.log(
		`Seeded descriptions (pipeline): ${result.seededDescriptionCount}`,
	);
	for (const seededDocument of result.seededDocuments) {
		console.log(
			`  Pipeline doc: ${seededDocument.objectKey} (${seededDocument.documentId})`,
		);
	}
	console.log(
		`Seeded documents (quality review): ${result.seededQRDocuments.length}`,
	);
	console.log(
		`Seeded descriptions (quality review): ${result.seededQRDescriptionCount}`,
	);
	for (const seededDocument of result.seededQRDocuments) {
		console.log(
			`  QR doc: ${seededDocument.objectKey} (${seededDocument.documentId})`,
		);
	}
	console.log(
		`Seeded documents (segmentation): ${result.seededSegmentationDocuments.length}`,
	);
	for (const seededDocument of result.seededSegmentationDocuments) {
		console.log(
			`  Segmentation doc: ${seededDocument.objectKey} (${seededDocument.documentId})`,
		);
	}
	console.log("\nAgent Graph Runs (quality review):");
	for (const [label, run] of Object.entries(result.agentRuns)) {
		console.log(`  ${label}: ${run.document} → ${run.verdict} (${run.id})`);
	}
};

seed()
	.catch((error) => {
		console.error("Seed failed", error);
		process.exitCode = 1;
	})
	.finally(async () => {
		await db.$client.end();
	});
