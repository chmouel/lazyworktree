// Theme
(function () {
  const root = document.documentElement;
  const toggle = document.querySelector(".theme-toggle");
  const prefersDark = window.matchMedia("(prefers-color-scheme: dark)");

  function applyTheme(theme) {
    root.dataset.theme = theme;
  }

  function getEffectiveTheme() {
    const stored = localStorage.getItem("theme");
    if (stored) return stored;
    return prefersDark.matches ? "dark" : "light";
  }

  applyTheme(getEffectiveTheme());

  if (toggle) {
    toggle.addEventListener("click", function () {
      var next = root.dataset.theme === "dark" ? "light" : "dark";
      localStorage.setItem("theme", next);
      applyTheme(next);
    });
  }

  prefersDark.addEventListener("change", function () {
    if (!localStorage.getItem("theme")) {
      applyTheme(prefersDark.matches ? "dark" : "light");
    }
  });
})();

const revealNodes = document.querySelectorAll(".reveal");

if ("IntersectionObserver" in window) {
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
} else {
  revealNodes.forEach((node) => {
    node.classList.add("visible");
  });
}

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

// Lightbox
const lightbox = document.getElementById("lightbox");
const lightboxImg = document.getElementById("lightbox-img");

function openLightbox(src, alt) {
  lightboxImg.src = src;
  lightboxImg.alt = alt;
  lightboxImg.classList.remove("zoomed");
  lightbox.hidden = false;
  document.body.style.overflow = "hidden";
}

function closeLightbox() {
  lightbox.hidden = true;
  lightboxImg.src = "";
  document.body.style.overflow = "";
}

document.querySelectorAll(".preview-card img").forEach((img) => {
  img.addEventListener("click", () => {
    openLightbox(img.src, img.alt);
  });
});

lightbox.addEventListener("click", (e) => {
  if (e.target === lightboxImg) {
    lightboxImg.classList.toggle("zoomed");
  } else {
    closeLightbox();
  }
});

document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && !lightbox.hidden) {
    closeLightbox();
  }
});

// Feature Modal
const featureModal = document.getElementById("feature-modal");
const featureModalBody = document.getElementById("feature-modal-body");
const featureModalCloseBtn = document.getElementById("feature-modal-close");
const featureModalBackdrop = document.getElementById("feature-modal-backdrop");
const featureCards = document.querySelectorAll(".feature-card[role='button']");
let activeFeatureCard = null;

function openFeatureModal(card) {
  const title = card.querySelector("h3").outerHTML;
  const details = card.querySelector(".feature-details").innerHTML;

  featureModalBody.innerHTML = title + details;
  featureModal.hidden = false;
  activeFeatureCard = card;

  // Trigger reflow to start animation
  void featureModal.offsetWidth;
  featureModal.classList.add("active");
  document.body.style.overflow = "hidden";
  featureModalCloseBtn.focus();
}

function closeFeatureModal() {
  featureModal.classList.remove("active");
  // Wait for transition to finish
  setTimeout(() => {
    if (!featureModal.classList.contains("active")) {
      featureModal.hidden = true;
      featureModalBody.innerHTML = "";
      document.body.style.overflow = "";
      if (activeFeatureCard) {
        activeFeatureCard.focus();
        activeFeatureCard = null;
      }
    }
  }, 300);
}

featureCards.forEach((card) => {
  card.addEventListener("click", () => openFeatureModal(card));
  card.addEventListener("keydown", (e) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      openFeatureModal(card);
    }
  });
});

if (featureModalCloseBtn) {
  featureModalCloseBtn.addEventListener("click", closeFeatureModal);
}
if (featureModalBackdrop) {
  featureModalBackdrop.addEventListener("click", closeFeatureModal);
}

document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && featureModal && !featureModal.hidden) {
    closeFeatureModal();
  }
});
