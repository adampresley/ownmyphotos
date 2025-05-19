document.addEventListener('DOMContentLoaded', () => {
   const searchInput = document.querySelector("#globalSearch");

   searchInput.addEventListener("input", (e) => {
      // If the search field is empty, navigate to the home page
      if (!e.target.value.trim()) {
         // Check if we're not already on the home page to avoid unnecessary navigation
         if (window.location.pathname !== '/' || window.location.search !== '') {
            window.location.href = `/?root=${window.root || ''}`;
         }
      }
   });

   function updateRootFromURL() {
      const urlParams = new URLSearchParams(window.location.search);
      const rootValue = urlParams.get('root') || '';

      // Update the #root element if it exists
      const rootElement = document.querySelector("#root");
      if (rootElement) {
         rootElement.value = rootValue;
      }

      // Also store in window.root for other scripts
      window.root = rootValue;
   }

   updateRootFromURL();

   // Listen for HTMX events that might change the URL
   document.body.addEventListener('htmx:afterSettle', updateRootFromURL);
   document.body.addEventListener('htmx:historyRestore', updateRootFromURL);
   document.body.addEventListener('htmx:afterSwap', updateRootFromURL);

   // Also update when browser history changes
   window.addEventListener('popstate', updateRootFromURL);
});

