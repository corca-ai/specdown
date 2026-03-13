(() => {
  // TOC group collapse/expand toggle.
  document.querySelectorAll('.toc-group-title').forEach((btn) => {
    btn.addEventListener('click', () => {
      const group = btn.closest('.toc-group');
      group.classList.toggle('expanded');
      btn.setAttribute('aria-expanded',
        group.classList.contains('expanded') ? 'true' : 'false');
    });
  });

  const resolve = (href) => {
    const id = decodeURIComponent(href.slice(1));
    return document.getElementById(id);
  };

  const allItems = Array.from(document.querySelectorAll('.toc-link[href^="#"]'))
    .map((link) => {
      const el = resolve(link.getAttribute('href'));
      return el ? { link, el } : null;
    })
    .filter(Boolean);

  if (!allItems.length) return;

  const h2Entries = Array.from(document.querySelectorAll('.toc-list > .toc-item[data-anchor]'))
    .map((li) => {
      const el = document.getElementById(li.getAttribute('data-anchor'));
      return el ? { li, el } : null;
    })
    .filter(Boolean);

  const stickyHeadings = Array.from(document.querySelectorAll('.spec-body :is(h2,h3,h4,h5,h6)'))
    .map(el => ({ el, top: parseFloat(getComputedStyle(el).top) || 0 }));
  let prevStuckLast = null;

  let frame = 0;

  const update = () => {
    frame = 0;

    let stuckLast = null;
    for (const item of stickyHeadings) {
      if (Math.abs(item.el.getBoundingClientRect().top - item.top) < 2) {
        stuckLast = item.el;
      }
    }
    if (prevStuckLast !== stuckLast) {
      prevStuckLast?.classList.remove('stuck-last');
      stuckLast?.classList.add('stuck-last');
      prevStuckLast = stuckLast;
    }

    const stickyBottom = stuckLast ? stuckLast.getBoundingClientRect().bottom : 0;
    const offset = window.scrollY + Math.max(stickyBottom + 20, 50);

    let active = allItems[0];
    for (const item of allItems) {
      if (item.el.offsetTop <= offset) { active = item; continue; }
      break;
    }
    for (const item of allItems) {
      item.link.classList.toggle('active', item === active);
    }

    let activeH2 = h2Entries[0];
    for (const entry of h2Entries) {
      if (entry.el.offsetTop <= offset) { activeH2 = entry; continue; }
      break;
    }
    for (const entry of h2Entries) {
      entry.li.classList.toggle('expanded', entry === activeH2);
    }
  };

  const schedule = () => {
    if (frame) return;
    frame = window.requestAnimationFrame(update);
  };

  window.addEventListener('scroll', schedule, { passive: true });
  window.addEventListener('resize', schedule);
  update();
})();
