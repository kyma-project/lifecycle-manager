# Documentation Guidelines

When writing documentation for this project, follow these rules:

### General Rules

Keep your documentation clear, precise, and easy to read. Use active voice instead of passive voice. Avoid future tense. When a word has multiple meanings in everyday English, prefer the unambiguous technical or plain-English alternative; for example: 
-	instead of "as" or "since", use "because" (causation), "while" (temporal), or "like" (comparison)
-	instead of "may not", use "might not" (possibility) or "must not" (prohibition)
-	instead of "should" (recommendation or requirement), use "we recommend" or "must"
-	use "can" for ability, "may" for permission
-	instead of "once" (temporal vs. conditional), use "after" or "when"
-	instead of latin abbreviations such as “i.e.” or “e.g.”, use English phrases such as “that means” or “for example; such as; like”
Avoid marketing lingo and words such as "allows you to" or "enables you to" (better: "you can") or "leverage, utilize" (better: "use") . Always state the purpose first, then the instruction (“To [purpose], [instruction].). Always state the condition first, then the instruction or conclusion (“If [condition], [instruction].”). Use full sentences to introduce lists. All list items must match a consistent pattern; they can't be parts of a running sentence. 

### Templates

Use the document templates from the [kyma-project template repository](https://github.com/kyma-project/template-repository/tree/main/docs/user/assets/templates):
- **concept.md** - for explaining foundational ideas and principles
- **task.md** - for step-by-step instructions and how-to guides
- **troubleshooting.md** - for diagnostic information and solutions to common issues
- **custom-resource.md** - for documenting custom resource configuration and usage

### Style and Terminology

Follow the [Kyma style and terminology guidelines](https://github.com/kyma-project/community/blob/main/docs/guidelines/content-guidelines/04-style-and-terminology.md):
- Use **active voice** and **present tense**
- Use **imperative mood** for instructions (no "please")
- Address readers as **"you"**, not "we" or "let's"
- Use **sentence case** for standard text; **Title Case** for component names and headings
- Use **CamelCase** for Kubernetes resources (for example, `ConfigMap`, `APIRule`)
- Always capitalize "Kubernetes"; never abbreviate it
- Do not capitalize "namespace"
- Prefer "must" for mandatory requirements and "can" for optional features
- Use "using" or "with" instead of "via"
- Use "connect/connection" instead of "integrate/integration"
- Use American English spelling
- Avoid parentheses; use lists instead
- Include serial commas

### Formatting

Follow the [Kyma formatting guidelines](https://github.com/kyma-project/community/blob/main/docs/guidelines/content-guidelines/03-formatting.md):
- Use **bold** for parameters, HTTP headers, events, roles, UI elements, and variables/placeholders
- Use `code font` for code examples, values, endpoints, filenames, paths, repository names, status codes, flags, and custom resources
- Use **ordered lists** for sequential procedures; **unordered lists** for non-sequential items
- Keep list items consistent in structure (all sentences, all fragments, or all questions — never mixed)
- Use action verbs and present tense in headings (for example, "Expose a Service")
- Use tables for comparisons and structured information
- Use callout panels: `[!NOTE]` for specific information, `[!WARNING]` for critical alerts, `[!TIP]` for helpful advice
- Break lengthy paragraphs into lists or tables for readability