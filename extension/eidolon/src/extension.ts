import * as vscode from "vscode";
import axios from "axios";

const API_BASE = "http://localhost:8080";
const DEBOUNCE_MS = 300;

let statusBar: vscode.StatusBarItem;
let debounceTimer: NodeJS.Timeout | undefined;

export function activate(context: vscode.ExtensionContext) {
    statusBar = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
    statusBar.text = "$(loading~spin) Eidolon";
    statusBar.tooltip = "Eidolon";
    statusBar.command = "eidolon.openDashboard";
    statusBar.show();
    context.subscriptions.push(statusBar);
    checkHealth();
    setInterval(checkHealth, 10000);

    const provider = vscode.languages.registerInlineCompletionItemProvider(
        { pattern: "**" },
        {
            provideInlineCompletionItems(document, position) {
                return new Promise((resolve) => {
                    if (debounceTimer) { clearTimeout(debounceTimer); }
                    debounceTimer = setTimeout(async () => {
                        const items = await getCompletion(document, position);
                        resolve(items);
                    }, DEBOUNCE_MS);
                });
            },
        }
    );
    context.subscriptions.push(provider);

    context.subscriptions.push(
        vscode.commands.registerCommand("eidolon.openDashboard", () => {
            const panel = vscode.window.createWebviewPanel(
                "eidolonDashboard", "Eidolon Dashboard",
                vscode.ViewColumn.Beside, { enableScripts: true }
            );
            panel.webview.html = getDashboardHTML();
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand("eidolon.triggerCompletion", async () => {
            const editor = vscode.window.activeTextEditor;
            if (!editor) { return; }
            await vscode.commands.executeCommand("editor.action.inlineSuggest.trigger");
        })
    );

    setStatusReady();
}

async function getCompletion(
    document: vscode.TextDocument,
    position: vscode.Position
): Promise<vscode.InlineCompletionList> {
    try {
        const offset = document.offsetAt(position);
        const content = document.getText();
        if (content.trim().length < 10) { return { items: [] }; }
        const requestId = `vsc-${Date.now()}`;
        setStatusLoading();
        const response = await axios.post(
            `${API_BASE}/api/keystroke`,
            {
                request_id: requestId,
                file_path: document.fileName,
                language_id: document.languageId,
                content: content,
                cursor_offset: offset,
                timestamp: Date.now(),
            },
            { timeout: 5000 }
        );
        if (response.data.status !== "queued") {
            setStatusReady();
            return { items: [] };
        }
        const completion = await pollCompletion(requestId);
        setStatusReady();
        if (!completion) { return { items: [] }; }
        const item = new vscode.InlineCompletionItem(
            completion,
            new vscode.Range(position, position)
        );
        return { items: [item] };
    } catch (_) {
        setStatusError();
        return { items: [] };
    }
}

async function pollCompletion(requestId: string, timeoutMs = 60000): Promise<string | null> {
    return new Promise((resolve) => {
        let text = "";
        const deadline = setTimeout(() => resolve(text || null), timeoutMs);
        axios.get(`${API_BASE}/api/completions/stream`, {
            responseType: "stream",
            timeout: timeoutMs,
        }).then((res) => {
            res.data.on("data", (chunk: Buffer) => {
                const lines = chunk.toString().split("\n");
                for (const line of lines) {
                    if (!line.startsWith("data: ")) { continue; }
                    try {
                        const data = JSON.parse(line.slice(6));
                        if (data.request_id !== requestId) { continue; }
                        text += data.token;
                        if (data.done) { clearTimeout(deadline); resolve(text); }
                    } catch (_) { }
                }
            });
            res.data.on("end", () => { clearTimeout(deadline); resolve(text || null); });
            res.data.on("error", () => { clearTimeout(deadline); resolve(null); });
        }).catch(() => { clearTimeout(deadline); resolve(null); });
    });
}

async function checkHealth() {
    try {
        await axios.get(`${API_BASE}/api/health`, { timeout: 3000 });
        setStatusReady();
    } catch {
        setStatusError();
    }
}

function setStatusReady() {
    statusBar.text = "$(sparkle) Eidolon";
    statusBar.tooltip = "Eidolon ready";
    statusBar.backgroundColor = undefined;
}

function setStatusLoading() {
    statusBar.text = "$(loading~spin) Eidolon";
    statusBar.tooltip = "Eidolon generating...";
}

function setStatusError() {
    statusBar.text = "$(warning) Eidolon";
    statusBar.tooltip = "Eidolon — server not reachable";
    statusBar.backgroundColor = new vscode.ThemeColor("statusBarItem.errorBackground");
}

function getDashboardHTML(): string {
    return `<!DOCTYPE html><html><head>
  <style>body{margin:0;background:#0d1117;}iframe{width:100%;height:100vh;border:none;}</style>
  </head><body><iframe src="http://localhost:3000"></iframe></body></html>`;
}

export function deactivate() {
    if (debounceTimer) { clearTimeout(debounceTimer); }
}