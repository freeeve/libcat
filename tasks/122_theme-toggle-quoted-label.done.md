# 122 -- theme-toggle.html renders its label with literal quotation marks

Filed from libcatalog-demo (seen live after adopting hugo/v0.5.0).

In `hugo/layouts/_partials/theme-toggle.html`, paint() sets:

    btn.textContent = d ? {{ i18n "lightMode" | default "Light mode" | jsonify }}
                        : {{ i18n "darkMode" | default "Dark mode" | jsonify }};

Inside `<script>`, html/template re-encodes the jsonify output as a JS string
literal (the exact trap head-seo.html documents for JSON-LD), so the value that
lands in the JS is `"\"Light mode\""` and the button shows `"Light mode"` --
quotes and all. The server-rendered initial label (plain HTML context) is fine;
the bug appears as soon as paint() runs, i.e. always.

Fix: append `| safeJS` to both interpolations, as head-seo.html already does.

The demo site is carrying a temporary shadow of this partial with exactly that
one-line fix (its `layouts/_partials/theme-toggle.html`); it will delete the
shadow when this ships.
