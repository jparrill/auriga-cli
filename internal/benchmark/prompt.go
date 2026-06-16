package benchmark

import (
	"fmt"
	"os"
)

const systemPrompt = `You are an expert frontend developer specializing in Astro static sites.
You will receive a project plan, source HTML, and benchmark data.
Your task is to generate a COMPLETE Astro website following the plan exactly.

CRITICAL OUTPUT FORMAT — for each file output:
--- FILE: path/to/file.ext ---
(complete file content)
--- END FILE ---

RULES:
1. Generate ALL files from the plan structure
2. Every file COMPLETE — no TODOs, no truncation
3. npm install && npm run dev must work
4. NEVER include sensitive data — use placeholders
5. Tokyo Night dark theme
6. BenchmarkTable filters MUST work (use stopPropagation)
7. Include package.json with Astro deps`

const formatFixPrompt = `CRITICAL FORMAT REQUIREMENT — READ THIS FIRST:
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

`

const buildFixPrompt = `Your generated Astro project failed to build. Here is the error:

%s

Fix the issue and regenerate ALL project files. Common problems:
- Do NOT use @astrojs/node — this is a STATIC site, no adapter needed
- Use astro@latest (^5.x), not old versions
- Do NOT use getStaticProps (that is Next.js, not Astro)
- The site field in astro.config.mjs must be a valid URL like 'https://example.github.io/auriga-lab'
- Do NOT set adapter: null — just omit the adapter field entirely
- All imports must match dependencies in package.json
- Use output: 'static' in astro.config.mjs

Regenerate the COMPLETE project using --- FILE: path --- / --- END FILE --- format.

=== ORIGINAL REQUIREMENTS ===

`

func BuildPrompt(planFile, sourceHTML, benchmarksJSON string) (string, error) {
	plan, err := os.ReadFile(planFile)
	if err != nil {
		return "", fmt.Errorf("cannot read plan: %w", err)
	}

	source, err := os.ReadFile(sourceHTML)
	if err != nil {
		return "", fmt.Errorf("cannot read source HTML: %w", err)
	}
	sourceStr := string(source)
	if len(sourceStr) > 50000 {
		sourceStr = sourceStr[:50000]
	}

	benchmarks, err := os.ReadFile(benchmarksJSON)
	if err != nil {
		return "", fmt.Errorf("cannot read benchmarks: %w", err)
	}

	return fmt.Sprintf("%s\n\n=== PROJECT PLAN ===\n%s\n\n=== SOURCE HTML ===\n%s\n\n=== BENCHMARK DATA ===\n%s\n\nGenerate the complete project now.",
		systemPrompt, string(plan), sourceStr, string(benchmarks)), nil
}

func BuildFormatRetryPrompt(originalPrompt string) string {
	return formatFixPrompt + originalPrompt
}

func BuildSensitiveRetryPrompt(originalPrompt string, violations []Violation) string {
	fix := "\n\n=== FIX REQUIRED ===\nSensitive data found:\n"
	for _, v := range violations {
		fix += fmt.Sprintf("  - %s in %s\n", v.Description, v.FilePath)
	}
	fix += "Replace ALL with placeholders. Regenerate ALL files.\n"
	return originalPrompt + fix
}

func BuildBuildRetryPrompt(originalPrompt, buildError string) string {
	return fmt.Sprintf(buildFixPrompt, buildError) + originalPrompt
}
