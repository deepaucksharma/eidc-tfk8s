# ⚙️  The `.bot/` Contract

**Purpose**  
This directory is **not** product code.  
It's a *tool-box* of prompts & templates that guide the AI assistant while it
writes *real* code and tests elsewhere in the repo.

**Ground rules**

1. **Read-only** at runtime – the assistant must *never* commit changes here.  
2. Prompts define *how* the bot speaks; templates define *what* it scaffolds.  
3. Keep everything short, explicit, and version-controlled in one place.

That's it. If you edit a prompt, open a normal PR; no special scripts needed.