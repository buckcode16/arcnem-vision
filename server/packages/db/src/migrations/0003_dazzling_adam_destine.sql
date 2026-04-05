CREATE TABLE "document_ocr_results" (
	"id" uuid PRIMARY KEY DEFAULT uuidv7() NOT NULL,
	"document_id" uuid NOT NULL,
	"model_id" uuid NOT NULL,
	"input" jsonb DEFAULT '{}' NOT NULL,
	"text" text NOT NULL,
	"avg_confidence" integer,
	"result" jsonb NOT NULL,
	"created_at" timestamp DEFAULT now() NOT NULL,
	"updated_at" timestamp DEFAULT now() NOT NULL
);
--> statement-breakpoint
ALTER TABLE "agent_graph_nodes" DROP CONSTRAINT "agent_graph_nodes_node_type_known";--> statement-breakpoint
ALTER TABLE "agent_graph_template_nodes" DROP CONSTRAINT "agent_graph_template_nodes_node_type_known";--> statement-breakpoint
ALTER TABLE "document_ocr_results" ADD CONSTRAINT "document_ocr_results_document_id_documents_id_fk" FOREIGN KEY ("document_id") REFERENCES "public"."documents"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "document_ocr_results" ADD CONSTRAINT "document_ocr_results_model_id_models_id_fk" FOREIGN KEY ("model_id") REFERENCES "public"."models"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "document_ocr_results_document_id_idx" ON "document_ocr_results" USING btree ("document_id");--> statement-breakpoint
CREATE INDEX "document_ocr_results_model_id_idx" ON "document_ocr_results" USING btree ("model_id");--> statement-breakpoint
CREATE INDEX "document_ocr_results_document_created_at_idx" ON "document_ocr_results" USING btree ("document_id","created_at");
