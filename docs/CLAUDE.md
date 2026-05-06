# Documentation Guidelines

When writing documentation for this project, follow these rules.

## General rules

Keep documentation clear, precise, and easy to read. Use active voice and present tense. Avoid future tense.

When a word has multiple meanings, prefer the unambiguous alternative:

| Avoid | Use instead |
|---|---|
| "as" / "since" | "because" (causation), "while" (temporal), "like" (comparison) |
| "may not" | "might not" (possibility) or "must not" (prohibition) |
| "should" | "we recommend" (recommendation) or "must" (requirement) |
| "once" (temporal vs. conditional) | "after" or "when" |
| "i.e." / "e.g." | "that means" / "for example" |
| "allows you to" / "enables you to" | "you can" |
| "leverage" / "utilize" | "use" |

Always state the purpose before the instruction: "To [purpose], [instruction]."
Always state the condition before the conclusion: "If [condition], [instruction]."

Use full sentences to introduce lists. All list items must follow a consistent pattern — never mix sentences and fragments in the same list.

## Templates

Use the document templates from the [kyma-project template repository](https://github.com/kyma-project/template-repository/tree/main/docs/user/assets/templates):

- **concept.md** — for explaining foundational ideas and principles
- **task.md** — for step-by-step instructions and how-to guides
- **troubleshooting.md** — for diagnostic information and solutions to common issues
- **custom-resource.md** — for documenting custom resource configuration and usage

## Style and terminology

Follow the [Kyma style and terminology guidelines](https://github.com/kyma-project/community/blob/main/docs/guidelines/content-guidelines/04-style-and-terminology.md):

- Use **imperative mood** for instructions — no "please"
- Address readers as "you", not "we" or "let's"
- Use **sentence case** for standard text; **Title Case** for component names and headings
- Use **CamelCase** for Kubernetes resources (`ConfigMap`, `APIRule`)
- Always capitalize "Kubernetes"; never abbreviate it
- Do not capitalize "namespace"
- "must" for mandatory requirements; "can" for optional features
- "using" or "with" instead of "via"
- "connect/connection" instead of "integrate/integration"
- American English spelling
- Avoid parentheses — use lists instead
- Include serial commas

## Formatting

Follow the [Kyma formatting guidelines](https://github.com/kyma-project/community/blob/main/docs/guidelines/content-guidelines/03-formatting.md):

- **Bold** for parameters, HTTP headers, events, roles, UI elements, and variables/placeholders
- `Code font` for code examples, values, endpoints, filenames, paths, repository names, status codes, flags, and custom resources
- **Ordered lists** for sequential procedures; **unordered lists** for non-sequential items
- Action verbs and present tense in headings (for example, "Expose a Service")
- Tables for comparisons and structured information
- Callout panels: `[!NOTE]` for specific information, `[!WARNING]` for critical alerts, `[!TIP]` for helpful advice
- Break lengthy paragraphs into lists or tables for readability
