#!/usr/bin/env python3
"""
Tichy Chat - Gradio UI for Tichy RAG Server

This app provides a chat interface that connects to the Tichy server
running on http://localhost:7070
"""

import json
from typing import List, Tuple

import gradio as gr
import requests

# Server configuration
SERVER_URL = "http://localhost:7070/v1/chat/completions"
MODEL = "gpt-4"


def chat(message: str, history: List[Tuple[str, str]]) -> str:
    """
    Send a message to the Tichy server and return the response.

    Args:
        message: The user's message
        history: List of (user_message, assistant_message) tuples

    Returns:
        The assistant's response
    """
    # Build message list from history
    messages = []
    for user_msg, assistant_msg in history:
        messages.append({"role": "user", "content": user_msg})
        messages.append({"role": "assistant", "content": assistant_msg})

    # Add current message
    messages.append({"role": "user", "content": message})

    # Call the server
    try:
        response = requests.post(
            SERVER_URL,
            json={
                "model": MODEL,
                "messages": messages
            },
            timeout=30
        )
        response.raise_for_status()

        data = response.json()
        return data["choices"][0]["message"]["content"]

    except requests.exceptions.ConnectionError:
        return (
            "❌ Error: Cannot connect to Tichy server. "
            "Make sure it's running on http://localhost:7070"
        )
    except requests.exceptions.Timeout:
        return "⏱️ Error: Request timed out. The server might be overloaded."
    except requests.exceptions.RequestException as e:
        return f"❌ Error: {str(e)}"
    except (KeyError, json.JSONDecodeError) as e:
        return f"❌ Error parsing response: {str(e)}"


# Create dark theme
theme = gr.themes.Soft(
    primary_hue="blue",
    secondary_hue="slate",
)

# Create Gradio interface
demo = gr.ChatInterface(
    fn=chat,
    title="Tichy Chat",
    description=(
        "Chat interface powered by Tichy RAG server. "
        "Make sure the server is running on http://localhost:7070"
    ),
    examples=[
        "What products does Insurellm offer?",
        "Tell me about the company history",
        "How many employees does Insurellm have?",
        "What is Markellm?",
        "Who founded Insurellm?",
    ],
    theme=theme,
)


if __name__ == "__main__":
    print("Starting Tichy Chat...")
    print("Make sure the Tichy server is running: ./tichy serve")
    print("Opening UI at http://localhost:7860")
    demo.launch(server_name="0.0.0.0", server_port=7860)
