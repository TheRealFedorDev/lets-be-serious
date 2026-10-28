/* ODIN motion layer. No scroll listeners; IO + MO only. Safe to fail: content is visible by default. */
(() => {
  "use strict";
  const doc = document;
  const reduce = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)");
  const canReveal = "IntersectionObserver" in window && "MutationObserver" in window;
  const TARGETS = [
    "#homeView .section-head",
    "#homeView .mood-card", "#homeView .deal-card", "#homeView .destination-card", "#homeView .hotel-card",
    "#homeView .ai-section", "#homeView .stats-section"
  ].join(",");
  const seen = new WeakSet();
  const pending = new Set();
  let io = null;

  function release(el) {
    el.classList.remove("m-pre");
    if (reduce && reduce.matches) return;
    el.classList.add("m-in");
    el.addEventListener("animationend", () => {
      el.classList.remove("m-in");
      el.style.removeProperty("--m-d");
    }, { once: true });
  }

  function onIntersect(entries) {
    let i = 0;
    entries.forEach((entry) => {
      if (!entry.isIntersecting) return;
      const el = entry.target;
      io.unobserve(el);
      pending.delete(el);
      el.style.setProperty("--m-d", Math.min(i++, 4) * 50 + "ms");
      release(el);
    });
  }

  function register(el) {
    if (seen.has(el) || el.classList.contains("skeleton-card")) return;
    seen.add(el);
    const r = el.getBoundingClientRect();
    // Not rendered (hidden view) or already in/above viewport: leave visible, no flash.
    if ((r.width === 0 && r.height === 0) || r.top < window.innerHeight) return;
    el.classList.add("m-pre");
    pending.add(el);
    io.observe(el);
  }

  function scan(root) {
    if (root.nodeType !== 1) return;
    if (root.matches(TARGETS)) register(root);
    root.querySelectorAll(TARGETS).forEach(register);
  }

  function initReveal() {
    if (!canReveal || (reduce && reduce.matches)) return;
    io = new IntersectionObserver(onIntersect, { rootMargin: "0px 0px -6% 0px", threshold: 0.05 });
    const home = doc.getElementById("homeView");
    if (!home) return;
    scan(home);
    new MutationObserver((muts) => muts.forEach((m) => m.addedNodes.forEach(scan)))
      .observe(home, { childList: true, subtree: true });
    // Failsafe: never leave content invisible.
    setTimeout(() => { pending.forEach((el) => { io.unobserve(el); release(el); }); pending.clear(); }, 12000);
    if (reduce && reduce.addEventListener) {
      reduce.addEventListener("change", () => {
        if (reduce.matches) { pending.forEach((el) => { io.unobserve(el); el.classList.remove("m-pre"); }); pending.clear(); }
      });
    }
  }

  function initViews() {
    if (!("MutationObserver" in window)) return;
    const views = doc.querySelectorAll(".view");
    const mo = new MutationObserver((records) => {
      if (reduce && reduce.matches) return;
      const first = new Map();
      records.forEach((r) => { if (!first.has(r.target)) first.set(r.target, r.oldValue || ""); });
      first.forEach((oldClass, view) => {
        const wasHidden = /(^|\s)hidden(\s|$)/.test(oldClass);
        if (!wasHidden || view.classList.contains("hidden")) return;
        view.classList.remove("m-view-in");
        void view.offsetWidth;
        view.classList.add("m-view-in");
        view.addEventListener("animationend", () => view.classList.remove("m-view-in"), { once: true });
      });
    });
    views.forEach((v) => mo.observe(v, { attributes: true, attributeFilter: ["class"], attributeOldValue: true }));
  }

  function init() { try { initReveal(); initViews(); } catch (e) { /* motion is optional */ } }
  if (doc.readyState === "loading") doc.addEventListener("DOMContentLoaded", init); else init();
})();
