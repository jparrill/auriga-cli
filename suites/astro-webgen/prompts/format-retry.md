CRITICAL FORMAT REQUIREMENT — READ THIS FIRST:
You MUST output every file using EXACTLY this format, with NO backticks around the content:

--- FILE: path/to/file.ext ---
file content here (raw, no backtick wrapper)
--- END FILE ---

Example:
--- FILE: package.json ---
{"name": "example"}
--- END FILE ---

--- FILE: src/pages/index.astro ---
---
import Layout from '../components/Layout.astro';
---
<Layout title="Home"><h1>Hello</h1></Layout>
--- END FILE ---

Do NOT wrap file contents in code blocks.
Do NOT add explanations between files.
Output ONLY --- FILE --- blocks, nothing else.

=== NOW GENERATE THE PROJECT ===

