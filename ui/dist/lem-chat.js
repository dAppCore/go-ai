// src/styles.ts
var chatStyles = `
  :host {
    display: flex;
    flex-direction: column;
    background: var(--lem-bg, #1a1a1e);
    color: var(--lem-text, #e0e0e0);
    font-family: var(--lem-font, system-ui, -apple-system, sans-serif);
    font-size: 14px;
    line-height: 1.5;
    border-radius: 12px;
    overflow: hidden;
    border: 1px solid rgba(255, 255, 255, 0.08);
  }

  .header {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 14px 18px;
    background: rgba(255, 255, 255, 0.03);
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);
    flex-shrink: 0;
  }

  .header-icon {
    width: 28px;
    height: 28px;
    border-radius: 8px;
    background: var(--lem-accent, #5865f2);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 14px;
    font-weight: 700;
    color: #fff;
  }

  .header-title {
    font-size: 15px;
    font-weight: 600;
    color: var(--lem-text, #e0e0e0);
  }

  .header-model {
    font-size: 11px;
    color: rgba(255, 255, 255, 0.35);
    margin-left: auto;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  }

  .header-status {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #43b581;
    flex-shrink: 0;
  }

  .header-status.disconnected {
    background: #f04747;
  }
`;
var messageStyles = `
  :host {
    display: block;
    padding: 6px 18px;
  }

  :host([role="user"]) .bubble {
    background: var(--lem-msg-user, #2a2a3e);
    margin-left: 40px;
    border-radius: 12px 12px 4px 12px;
  }

  :host([role="assistant"]) .bubble {
    background: var(--lem-msg-assistant, #1e1e2a);
    margin-right: 40px;
    border-radius: 12px 12px 12px 4px;
  }

  .bubble {
    padding: 10px 14px;
    word-wrap: break-word;
    overflow-wrap: break-word;
  }

  .role {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 4px;
    color: rgba(255, 255, 255, 0.35);
  }

  :host([role="assistant"]) .role {
    color: var(--lem-accent, #5865f2);
  }

  .content {
    color: var(--lem-text, #e0e0e0);
    line-height: 1.6;
  }

  .content p {
    margin: 0 0 8px 0;
  }

  .content p:last-child {
    margin-bottom: 0;
  }

  .content strong {
    font-weight: 600;
    color: #fff;
  }

  .content em {
    font-style: italic;
    color: rgba(255, 255, 255, 0.8);
  }

  .content code {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 12px;
    background: rgba(0, 0, 0, 0.3);
    padding: 2px 5px;
    border-radius: 4px;
    color: #e8a0bf;
  }

  .content pre {
    margin: 8px 0;
    padding: 12px;
    background: rgba(0, 0, 0, 0.35);
    border-radius: 8px;
    overflow-x: auto;
    border: 1px solid rgba(255, 255, 255, 0.06);
  }

  .content pre code {
    background: none;
    padding: 0;
    font-size: 12px;
    color: #c9d1d9;
    line-height: 1.5;
  }

  .think-panel {
    margin: 6px 0 8px;
    padding: 8px 12px;
    background: rgba(88, 101, 242, 0.06);
    border-left: 2px solid rgba(88, 101, 242, 0.3);
    border-radius: 0 6px 6px 0;
    font-size: 12px;
    color: rgba(255, 255, 255, 0.45);
    line-height: 1.5;
    max-height: 200px;
    overflow-y: auto;
  }

  .think-panel::-webkit-scrollbar {
    width: 4px;
  }

  .think-panel::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.1);
    border-radius: 2px;
  }

  .think-label {
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: rgba(88, 101, 242, 0.5);
    margin-bottom: 4px;
    cursor: pointer;
    user-select: none;
  }

  .think-label:hover {
    color: rgba(88, 101, 242, 0.7);
  }

  .think-panel.collapsed .think-content {
    display: none;
  }

  .cursor {
    display: inline-block;
    width: 7px;
    height: 16px;
    background: var(--lem-accent, #5865f2);
    border-radius: 1px;
    animation: blink 0.8s step-end infinite;
    vertical-align: text-bottom;
    margin-left: 2px;
  }

  @keyframes blink {
    50% { opacity: 0; }
  }
`;

// src/markdown.ts
function escapeHtml(text) {
  return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}
function parseInline(text) {
  let result = escapeHtml(text);
  result = result.replace(/`([^`]+)`/g, "<code>$1</code>");
  result = result.replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>");
  result = result.replace(/__(.+?)__/g, "<strong>$1</strong>");
  result = result.replace(/(?<!\w)\*([^*]+)\*(?!\w)/g, "<em>$1</em>");
  result = result.replace(/(?<!\w)_([^_]+)_(?!\w)/g, "<em>$1</em>");
  return result;
}
function wrapParagraph(lines) {
  const joined = lines.join("<br>");
  if (joined.startsWith("<pre")) return joined;
  return `<p>${joined}</p>`;
}
function renderMarkdown(text) {
  const lines = text.split("\n");
  const output = [];
  let inCodeBlock = false;
  let codeLines = [];
  let codeLang = "";
  for (const line of lines) {
    if (line.trimStart().startsWith("```")) {
      if (!inCodeBlock) {
        inCodeBlock = true;
        codeLang = line.trimStart().slice(3).trim();
        codeLines = [];
      } else {
        const langAttr = codeLang ? ` data-lang="${escapeHtml(codeLang)}"` : "";
        output.push(`<pre${langAttr}><code>${escapeHtml(codeLines.join("\n"))}</code></pre>`);
        inCodeBlock = false;
        codeLines = [];
        codeLang = "";
      }
      continue;
    }
    if (inCodeBlock) {
      codeLines.push(line);
      continue;
    }
    if (line.trim() === "") {
      output.push("");
      continue;
    }
    output.push(parseInline(line));
  }
  if (inCodeBlock) {
    const langAttr = codeLang ? ` data-lang="${escapeHtml(codeLang)}"` : "";
    output.push(`<pre${langAttr}><code>${escapeHtml(codeLines.join("\n"))}</code></pre>`);
  }
  const paragraphs = [];
  let current = [];
  for (const line of output) {
    if (line === "") {
      if (current.length > 0) {
        paragraphs.push(wrapParagraph(current));
        current = [];
      }
    } else {
      current.push(line);
    }
  }
  if (current.length > 0) {
    paragraphs.push(wrapParagraph(current));
  }
  return paragraphs.join("");
}

// src/lem-message.ts
var LemMessage = class extends HTMLElement {
  shadow;
  thinkPanel;
  thinkContent;
  thinkLabel;
  contentEl;
  cursorEl = null;
  _text = "";
  _streaming = false;
  _thinkCollapsed = false;
  constructor() {
    super();
    this.shadow = this.attachShadow({ mode: "open" });
  }
  connectedCallback() {
    const role = this.getAttribute("role") || "user";
    const style = document.createElement("style");
    style.textContent = messageStyles;
    const bubble = document.createElement("div");
    bubble.className = "bubble";
    const roleEl = document.createElement("div");
    roleEl.className = "role";
    roleEl.textContent = role === "assistant" ? "LEM" : "You";
    this.thinkPanel = document.createElement("div");
    this.thinkPanel.className = "think-panel";
    this.thinkPanel.style.display = "none";
    this.thinkLabel = document.createElement("div");
    this.thinkLabel.className = "think-label";
    this.thinkLabel.textContent = "\u25BC reasoning";
    this.thinkLabel.addEventListener("click", () => {
      this._thinkCollapsed = !this._thinkCollapsed;
      this.thinkPanel.classList.toggle("collapsed", this._thinkCollapsed);
      this.thinkLabel.textContent = this._thinkCollapsed ? "\u25B6 reasoning" : "\u25BC reasoning";
    });
    this.thinkContent = document.createElement("div");
    this.thinkContent.className = "think-content";
    this.thinkPanel.appendChild(this.thinkLabel);
    this.thinkPanel.appendChild(this.thinkContent);
    this.contentEl = document.createElement("div");
    this.contentEl.className = "content";
    bubble.appendChild(roleEl);
    if (role === "assistant") {
      bubble.appendChild(this.thinkPanel);
    }
    bubble.appendChild(this.contentEl);
    this.shadow.appendChild(style);
    this.shadow.appendChild(bubble);
    if (this._text) {
      this.updateContent();
    }
  }
  get text() {
    return this._text;
  }
  set text(value) {
    this._text = value;
    this.updateContent();
  }
  get streaming() {
    return this._streaming;
  }
  set streaming(value) {
    this._streaming = value;
    this.updateContent();
  }
  appendToken(token) {
    this._text += token;
    this.updateContent();
  }
  /**
   * Splits text into think/response portions and renders each.
   *
   * Safety: renderMarkdown() escapes all HTML entities (& < > ") before any
   * inline formatting is applied. The source is the local MLX model output,
   * not arbitrary user HTML. Shadow DOM provides additional isolation.
   */
  updateContent() {
    if (!this.contentEl) return;
    const { think, response } = this.splitThink(this._text);
    if (think !== null && this.thinkPanel) {
      this.thinkPanel.style.display = "";
      this.thinkContent.textContent = think;
    }
    const responseHtml = renderMarkdown(response);
    this.contentEl.innerHTML = responseHtml;
    if (this._streaming) {
      if (!this.cursorEl) {
        this.cursorEl = document.createElement("span");
        this.cursorEl.className = "cursor";
      }
      if (think !== null && !this._text.includes("</think>")) {
        this.thinkContent.appendChild(this.cursorEl);
      } else {
        const lastChild = this.contentEl.lastElementChild || this.contentEl;
        lastChild.appendChild(this.cursorEl);
      }
    }
  }
  splitThink(text) {
    const thinkStart = text.indexOf("<think>");
    if (thinkStart === -1) {
      return { think: null, response: text };
    }
    const afterOpen = thinkStart + "<think>".length;
    const thinkEnd = text.indexOf("</think>", afterOpen);
    if (thinkEnd === -1) {
      return {
        think: text.slice(afterOpen).trim(),
        response: text.slice(0, thinkStart).trim()
      };
    }
    const thinkText = text.slice(afterOpen, thinkEnd).trim();
    const beforeThink = text.slice(0, thinkStart).trim();
    const afterThink = text.slice(thinkEnd + "</think>".length).trim();
    const response = [beforeThink, afterThink].filter(Boolean).join("\n");
    return { think: thinkText, response };
  }
};
customElements.define("lem-message", LemMessage);

// src/lem-chat.ts
var LemChat = class extends HTMLElement {
  shadow;
  messages;
  input;
  statusEl;
  history = [];
  abortController = null;
  static get observedAttributes() {
    return ["endpoint", "model", "system-prompt", "max-tokens", "temperature"];
  }
  constructor() {
    super();
    this.shadow = this.attachShadow({ mode: "open" });
  }
  connectedCallback() {
    const style = document.createElement("style");
    style.textContent = chatStyles;
    const header = document.createElement("div");
    header.className = "header";
    this.statusEl = document.createElement("div");
    this.statusEl.className = "header-status";
    const icon = document.createElement("div");
    icon.className = "header-icon";
    icon.textContent = "L";
    const title = document.createElement("div");
    title.className = "header-title";
    title.textContent = "LEM";
    const modelLabel = document.createElement("div");
    modelLabel.className = "header-model";
    modelLabel.textContent = this.getAttribute("model") || "local";
    header.appendChild(this.statusEl);
    header.appendChild(icon);
    header.appendChild(title);
    header.appendChild(modelLabel);
    this.messages = document.createElement("lem-messages");
    this.input = document.createElement("lem-input");
    this.shadow.appendChild(style);
    this.shadow.appendChild(header);
    this.shadow.appendChild(this.messages);
    this.shadow.appendChild(this.input);
    this.addEventListener("lem-send", ((e) => {
      const detail = e.detail;
      this.handleSend(detail.text);
    }));
    const systemPrompt = this.getAttribute("system-prompt");
    if (systemPrompt) {
      this.history.push({ role: "system", content: systemPrompt });
    }
    this.checkConnection();
    requestAnimationFrame(() => this.input.focus());
  }
  disconnectedCallback() {
    this.abortController?.abort();
  }
  get endpoint() {
    const attr = this.getAttribute("endpoint");
    if (!attr) return window.location.origin;
    return attr;
  }
  get model() {
    return this.getAttribute("model") || "";
  }
  get maxTokens() {
    const val = this.getAttribute("max-tokens");
    return val ? parseInt(val, 10) : 2048;
  }
  get temperature() {
    const val = this.getAttribute("temperature");
    return val ? parseFloat(val) : 0.7;
  }
  async checkConnection() {
    try {
      const resp = await fetch(`${this.endpoint}/v1/models`, {
        signal: AbortSignal.timeout(3e3)
      });
      this.statusEl.classList.toggle("disconnected", !resp.ok);
    } catch {
      this.statusEl.classList.add("disconnected");
    }
  }
  async handleSend(text) {
    this.messages.addMessage("user", text);
    this.history.push({ role: "user", content: text });
    const assistantMsg = this.messages.addMessage("assistant");
    assistantMsg.streaming = true;
    this.input.disabled = true;
    this.abortController?.abort();
    this.abortController = new AbortController();
    let fullResponse = "";
    try {
      const response = await fetch(`${this.endpoint}/v1/chat/completions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        signal: this.abortController.signal,
        body: JSON.stringify({
          model: this.model,
          messages: this.history,
          max_tokens: this.maxTokens,
          temperature: this.temperature,
          stream: true
        })
      });
      if (!response.ok) {
        throw new Error(`Server error: ${response.status}`);
      }
      if (!response.body) {
        throw new Error("No response body");
      }
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";
        for (const line of lines) {
          if (!line.startsWith("data: ")) continue;
          const data = line.slice(6).trim();
          if (data === "[DONE]") continue;
          try {
            const chunk = JSON.parse(data);
            const delta = chunk.choices?.[0]?.delta;
            if (delta?.content) {
              fullResponse += delta.content;
              assistantMsg.appendToken(delta.content);
              this.messages.scrollToBottom();
            }
          } catch {
          }
        }
      }
    } catch (err) {
      if (err instanceof Error && err.name === "AbortError") {
      } else {
        const errorText = err instanceof Error ? err.message : "Connection failed";
        if (!fullResponse) {
          assistantMsg.text = `\u26A0\uFE0F ${errorText}`;
        }
        this.statusEl.classList.add("disconnected");
      }
    } finally {
      assistantMsg.streaming = false;
      this.input.disabled = false;
      this.input.focus();
      this.abortController = null;
      if (fullResponse) {
        this.history.push({ role: "assistant", content: fullResponse });
      }
    }
  }
};
customElements.define("lem-chat", LemChat);
export {
  LemChat
};
