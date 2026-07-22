  document.querySelectorAll(".copy-btn").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var flash = function (msg) {
        var prev = btn.textContent;
        btn.textContent = msg;
        setTimeout(function () { btn.textContent = prev; }, 1600);
      };
      var selectCommand = function () {
        var code = btn.parentElement.querySelector("code");
        var range = document.createRange();
        range.selectNodeContents(code);
        var sel = window.getSelection();
        sel.removeAllRanges();
        sel.addRange(range);
        flash("Press ⌘C");
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(btn.dataset.copy)
          .then(function () { flash("Copied"); })
          .catch(selectCommand);
      } else {
        selectCommand();
      }
    });
  });

  /* The .js class arms the reveal-hiding CSS, so it is only added when an
     observer is guaranteed to un-hide the sections. */
  if (!window.matchMedia("(prefers-reduced-motion: reduce)").matches && "IntersectionObserver" in window) {
    document.documentElement.classList.add("js");
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (e) {
        if (e.isIntersecting) { e.target.classList.add("in"); io.unobserve(e.target); }
      });
    }, { rootMargin: "0px 0px -8% 0px" });
    document.querySelectorAll(".reveal").forEach(function (el) { io.observe(el); });
  }
