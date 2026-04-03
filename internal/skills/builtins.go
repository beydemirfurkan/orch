package skills

// DefaultRegistry returns a Registry pre-loaded with the built-in skills.
// All built-in skills are disabled by default; enable them via config.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	for _, s := range builtinSkills() {
		r.Register(s)
	}
	return r
}

func builtinSkills() []*Skill {
	return []*Skill{
		{
			Name:        "cartography",
			Description: "Generates a hierarchical codemap of the repository — entry points, packages, key symbols.",
			Tools:       []string{"list_files", "read_file"},
			SystemHint:  "Before planning, build a mental map of the repository structure: identify entry points, package boundaries, and key exported symbols relevant to the task.",
			Enabled:     false,
		},
		{
			Name:        "simplify",
			Description: "Post-generation quality pass: remove redundant code, consolidate duplicated logic.",
			Tools:       []string{"read_file", "search_code"},
			SystemHint:  "After generating the patch, review for unnecessary complexity: remove unused imports, deduplicate repeated logic, and prefer the simplest correct solution.",
			Enabled:     false,
		},
		{
			Name:        "web_search",
			Description: "Retrieves external documentation and code examples to inform decisions.",
			Tools:       []string{"http_fetch"},
			SystemHint:  "When uncertain about an API, library behaviour, or best practice, use the available search tools to retrieve authoritative documentation before generating code.",
			Enabled:     false,
		},
		{
			Name:        "test_scout",
			Description: "Finds test files by naming convention AND by searching for function names in modified files.",
			Tools:       []string{"search_code", "read_file"},
			SystemHint:  "Actively identify test files that exercise the code you are modifying. Look for both naming-convention matches (_test.go, .spec.ts) and files that import or call the functions you change.",
			Enabled:     false,
		},
		{
			Name:        "context7",
			Description: "Queries the context7 MCP server for up-to-date library documentation.",
			Tools:       []string{"context7_query"},
			SystemHint:  "Use context7 to look up current API documentation for any third-party library before using it. This prevents hallucinated or outdated API calls.",
			Enabled:     false,
		},
		{
			Name:        "code_search",
			Description: "Searches code across the repository for usage patterns and existing implementations.",
			Tools:       []string{"search_code"},
			SystemHint:  "Search the codebase for existing implementations of similar functionality before writing new code. Prefer extending proven patterns over inventing new ones.",
			Enabled:     false,
		},
	}
}
