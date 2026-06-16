The project you generated contains sensitive data that must be removed.

Detected violations:
{{range .Violations}}- {{.Description}} found in {{.FilePath}}
{{end}}

Review the project plan's sensitive data rules and fix ALL violations.
Replace real values with the appropriate placeholders (<server-ip>, <tailscale-ip>, <your-telegram-token>, etc.).

Here are the files that need fixing:
{{range .AffectedFiles}}
--- CURRENT FILE: {{.Path}} ---
{{.Content}}
--- END CURRENT FILE ---
{{end}}

Generate ONLY the fixed files using --- FILE: path --- / --- END FILE --- format.
Do NOT regenerate files that have no violations.
