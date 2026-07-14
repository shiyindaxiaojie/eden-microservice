import { appendFileSync, existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import readline from "node:readline";

const root = resolve(process.env.EDEN_ENGINEERING_ROOT || process.cwd());
const pitfallsPath = resolve(root, "specs/zh-CN/engineering/agent-pitfalls.md");
const tools = [
  {
    name: "eden_task_context",
    description: "Return the repository engineering contract and available focused skills.",
    inputSchema: { type: "object", properties: {} }
  },
  {
    name: "eden_search_pitfalls",
    description: "Search confirmed, reusable agent pitfalls before making a change.",
    inputSchema: {
      type: "object",
      properties: { query: { type: "string" } },
      required: ["query"]
    }
  },
  {
    name: "eden_record_pitfall",
    description: "Record a confirmed reusable mistake after a user correction, test failure, or review finding.",
    inputSchema: {
      type: "object",
      properties: {
        id: { type: "string" },
        trigger: { type: "string" },
        prevention: { type: "string" }
      },
      required: ["id", "trigger", "prevention"]
    }
  }
];

function text(value) {
  return { content: [{ type: "text", text: value }] };
}

function register() {
  if (!existsSync(pitfallsPath)) {
    throw new Error("Pitfall register is missing: " + pitfallsPath);
  }
  return readFileSync(pitfallsPath, "utf8");
}

function callTool(name, args) {
  if (name === "eden_task_context") {
    return text(JSON.stringify({
      root,
      instructions: "AGENTS.md",
      specs: "specs/",
      skill: "skills/eden-microservice/SKILL.md",
      pitfalls: "specs/zh-CN/engineering/agent-pitfalls.md"
    }, null, 2));
  }
  if (name === "eden_search_pitfalls") {
    const query = String(args.query || "").trim().toLowerCase();
    const matches = register().split("\n\n### ").filter((section) => section.toLowerCase().includes(query));
    return text(matches.length ? matches.join("\n\n### ") : "No matching confirmed pitfall.");
  }
  if (name === "eden_record_pitfall") {
    const id = String(args.id).trim();
    const trigger = String(args.trigger).trim();
    const prevention = String(args.prevention).trim();
    if (!id || !trigger || !prevention) throw new Error("id, trigger, and prevention are required.");
    const current = register();
    if (current.includes("### " + id + ":")) throw new Error("Pitfall id already exists: " + id);
    appendFileSync(pitfallsPath, "\n### " + id + ": " + trigger + "\n\n- Prevention: " + prevention + "\n", "utf8");
    return text("Recorded confirmed pitfall " + id + ".");
  }
  throw new Error("Unknown tool: " + name);
}

function respond(message) {
  process.stdout.write(JSON.stringify(message) + "\n");
}

const input = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
input.on("line", (line) => {
  try {
    const request = JSON.parse(line);
    if (request.method === "initialize") {
      respond({ jsonrpc: "2.0", id: request.id, result: { protocolVersion: "2024-11-05", capabilities: { tools: {} }, serverInfo: { name: "eden-microservice-engineering", version: "0.1.0" } } });
      return;
    }
    if (request.method === "tools/list") {
      respond({ jsonrpc: "2.0", id: request.id, result: { tools } });
      return;
    }
    if (request.method === "tools/call") {
      respond({ jsonrpc: "2.0", id: request.id, result: callTool(request.params.name, request.params.arguments || {}) });
      return;
    }
    if (request.id !== undefined) respond({ jsonrpc: "2.0", id: request.id, error: { code: -32601, message: "Method not found" } });
  } catch (error) {
    respond({ jsonrpc: "2.0", id: null, error: { code: -32000, message: error.message } });
  }
});
