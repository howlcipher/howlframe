# Issue: HowlBoard `set_html` XSS Vulnerability

## Exact Data Path
1. The user inputs task data (Title, Description, Tags) in the browser.
2. `frontend/app.howl`'s `add_task` constructs a JSON payload via string concatenation (also vulnerable to JSON injection) and sends it to `/api/tasks/create`.
3. The backend stores this unescaped string data in the `memory://howlboard` store.
4. When tasks are fetched, `frontend/app.howl`'s `load_tasks` calls `render_task_card(task)`.
5. `render_task_card` constructs HTML via string concatenation:
   `(set html (+ html (+ "<div class=\"task-title\">" (+ title "</div>"))))`
6. This raw, untrusted string is passed directly to the DOM via `set_html (dom_query "#tasks-...") html`.

## Risk
High. A malicious user can input a task title such as `<script>alert('XSS')</script>` or `<img src=x onerror=alert(1)>`. When this is passed to `set_html`, the browser executes the payload in the context of the HowlBoard origin, compromising the user session and enabling arbitrary actions on their behalf.

## Why `set_html` is Inherently Dangerous
`set_html` maps directly to `Element.innerHTML`. When used with untrusted data, it skips all context-aware output encoding. Because HowlFrame currently lacks native HTML encoding or structured DOM builders, string concatenation into `innerHTML` is the path of least resistance, making XSS almost guaranteed in web applications that handle user input.

## Possible Future Safe Directions
1. **Explicit `html_escape` Primitive**: Introduce an `html_escape` construct in HowlFrame and require its use before interpolating untrusted data into HTML strings.
2. **Structured DOM Element Construction**: Provide HowlFrame primitives for safe DOM manipulation (e.g., `create_element`, `set_attribute`, `append_child`) rather than writing raw HTML strings.
3. **Safe Text APIs**: Expand the use of `set_text` (`textContent`) and encourage data-binding models where values are safely inserted as text nodes rather than parsed as HTML markup.
