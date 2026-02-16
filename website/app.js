const revealNodes = document.querySelectorAll(".reveal");

const revealObserver = new IntersectionObserver(
  (entries) => {
    entries.forEach((entry) => {
      if (entry.isIntersecting) {
        entry.target.classList.add("visible");
        revealObserver.unobserve(entry.target);
      }
    });
  },
  { threshold: 0.15 }
);

revealNodes.forEach((node, index) => {
  node.style.transitionDelay = `${Math.min(index * 80, 350)}ms`;
  revealObserver.observe(node);
});

const tabButtons = document.querySelectorAll(".tab-btn");
const tabPanels = document.querySelectorAll(".tab-panel");

tabButtons.forEach((button) => {
  button.addEventListener("click", () => {
    tabButtons.forEach((item) => {
      item.classList.remove("active");
      item.setAttribute("aria-selected", "false");
    });
    tabPanels.forEach((panel) => {
      panel.classList.remove("active");
      panel.hidden = true;
    });

    button.classList.add("active");
    button.setAttribute("aria-selected", "true");
    const panel = document.getElementById(button.dataset.target);
    if (!panel) {
      return;
    }
    panel.classList.add("active");
    panel.hidden = false;
  });
});
