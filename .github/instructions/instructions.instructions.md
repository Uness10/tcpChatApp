# Prompting Guidelines for Each File

## Understand the Project Context

- Analyze existing files and infer project structure.
- Follow architectural patterns already used (e.g., MVC, layered architecture, event-driven, etc.).
- Maintain consistent naming conventions, folder structure, and abstraction levels.

##  Use Common Sense and Reasoning

- Avoid redundant logic or overengineering.
- Anticipate edge cases in logic (e.g., null values, empty arrays, invalid inputs).
- Avoid suggesting anti-patterns or insecure practices.

##  Function Implementation Instructions

- Include type annotations (if supported).
- Prioritize clarity, performance, and modularity.
- Break down large functions into smaller, reusable pieces.
- Suggest improvements or refactors if the logic can be optimized.

##  Security & Robustness

- Validate inputs thoroughly.
- Avoid hardcoded credentials or insecure patterns.
- In web/backend projects: prevent common vulnerabilities (SQLi, XSS, CSRF, etc.).

##  Dependencies & Imports

- Use only well-maintained, documented libraries.
- Avoid bloating the code with unnecessary packages.
