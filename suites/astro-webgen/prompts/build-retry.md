The project you generated failed to build. Here is the error:

```
{{.Error}}
```

Common problems:
- Do NOT use @astrojs/node — this is a STATIC site, no adapter needed
- Use astro@latest (^5.x), not old versions
- Do NOT use getStaticProps (that is Next.js, not Astro)
- The site field in astro.config.mjs must be a valid URL like 'https://example.github.io/auriga-lab'
- Do NOT set adapter: null — just omit the adapter field entirely
- All imports must match dependencies in package.json
- Use output: 'static' in astro.config.mjs

Here are the relevant files:
{{range .AffectedFiles}}
--- CURRENT FILE: {{.Path}} ---
{{.Content}}
--- END CURRENT FILE ---
{{end}}

Fix the issue and generate ONLY the files that need to change.
Use --- FILE: path --- / --- END FILE --- format.
Do NOT regenerate files that are not related to the error.
