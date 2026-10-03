(() => {
  const root = document.getElementById("swagger-ui");
  if (!root) return;

  // Swagger keeps every status in the contract; reveal non-200 rows on demand.
  const addResponseToggles = () => {
    for (const table of root.querySelectorAll(".responses-table")) {
      const count = table.querySelectorAll('tbody tr[data-code]:not([data-code="200"])').length;
      if (!count || table.nextElementSibling?.classList.contains("api-responses-toggle")) continue;
      const button = document.createElement("button");
      button.type = "button";
      button.className = "api-responses-toggle";
      button.setAttribute("aria-controls", table.id);
      const update = () => {
        const expanded = table.classList.contains("api-show-errors");
        button.setAttribute("aria-expanded", String(expanded));
        button.textContent = expanded ? "收起其他响应" : `展开其他响应（${count}）`;
      };
      button.addEventListener("click", () => {
        table.classList.toggle("api-show-errors");
        update();
      });
      update();
      table.after(button);
    }
  };
  new MutationObserver(addResponseToggles).observe(root, { childList: true, subtree: true });

  SwaggerUIBundle({
    dom_id: "#swagger-ui",
    url: root.dataset.specUrl,
    deepLinking: true,
    docExpansion: "list",
    defaultModelsExpandDepth: 0,
    defaultModelExpandDepth: 1,
    filter: true,
    displayOperationId: false,
    supportedSubmitMethods: [],
    persistAuthorization: false,
    validatorUrl: null,
    presets: [SwaggerUIBundle.presets.apis],
  });
})();
