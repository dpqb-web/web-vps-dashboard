((doc) => {
  ["-webkit-", "-moz-", "-ms-", "-khtml-", ""].forEach((pre) =>
    doc.style.setProperty(`${pre}user-select`, "none")
  );

  doc.style.cursor = "default";
})(document.documentElement);

document.addEventListener("contextmenu", function (e) {
  e.preventDefault();
  return false;
});
