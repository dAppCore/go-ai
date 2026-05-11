# SPDX-License-Identifier: EUPL-1.2

"""Gradio client for the go-ai book-state teacher/student demo.

The model work is performed by the Go process. This file only provides a local
UI that calls POST /ask on go/cmd/book-state-demo or any compatible host.
"""

from __future__ import annotations

import json
import os
import urllib.error
import urllib.request

import gradio as gr


DEFAULT_ENDPOINT = os.getenv("CORE_BOOK_STATE_DEMO_URL", "http://127.0.0.1:8787")


def _post_json(endpoint: str, path: str, payload: dict) -> dict:
    base = endpoint.rstrip("/")
    data = json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(
        f"{base}{path}",
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=300) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8")
        try:
            parsed = json.loads(detail)
            message = parsed.get("error", detail)
        except json.JSONDecodeError:
            message = detail
        raise gr.Error(message) from exc
    except urllib.error.URLError as exc:
        raise gr.Error(f"Go demo endpoint is unavailable: {exc}") from exc


def ask_book_state(endpoint: str, question: str, max_tokens: int, student_uses_state: bool):
    question = question.strip()
    if not question:
        raise gr.Error("Question is required")

    payload = {
        "question": question,
        "max_tokens": int(max_tokens),
        "student_uses_book_state": bool(student_uses_state),
    }
    response = _post_json(endpoint, "/ask", payload)

    student_answer = response.get("student_answer", "").strip()
    teacher_answer = response.get("teacher_answer", "").strip()
    state = response.get("state", {})

    messages = [{"role": "user", "content": question}]
    if student_answer:
        messages.append({"role": "assistant", "content": f"Student:\n{student_answer}"})
    messages.append({"role": "assistant", "content": f"Teacher:\n{teacher_answer}"})

    state_summary = {
        "title": state.get("title"),
        "entry_uri": state.get("entry_uri"),
        "bundle_uri": state.get("bundle_uri"),
        "prefix_tokens": state.get("prefix_tokens"),
        "bundle_tokens": state.get("bundle_tokens"),
    }
    return messages, response, state_summary


with gr.Blocks(title="go-ai Book State Demo") as demo:
    gr.Markdown("# go-ai book state teacher/student")
    with gr.Row():
        endpoint = gr.Textbox(label="Go endpoint", value=DEFAULT_ENDPOINT, scale=2)
        max_tokens = gr.Slider(16, 1024, value=256, step=16, label="Max tokens")
        student_uses_state = gr.Checkbox(value=False, label="Student uses book state")

    question = gr.Textbox(
        label="Student question",
        lines=3,
        placeholder="What did Marcus learn from his grandfather Verus?",
    )
    ask = gr.Button("Ask", variant="primary")

    chat = gr.Chatbot(label="Teacher/student trace", type="messages", height=420)
    with gr.Row():
        raw = gr.JSON(label="Raw go-ai response")
        state = gr.JSON(label="State")

    ask.click(
        fn=ask_book_state,
        inputs=[endpoint, question, max_tokens, student_uses_state],
        outputs=[chat, raw, state],
    )
    question.submit(
        fn=ask_book_state,
        inputs=[endpoint, question, max_tokens, student_uses_state],
        outputs=[chat, raw, state],
    )


if __name__ == "__main__":
    demo.launch(
        server_name=os.getenv("GRADIO_SERVER_NAME", "127.0.0.1"),
        server_port=int(os.getenv("GRADIO_SERVER_PORT", "7860")),
    )
