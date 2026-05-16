(function() {
    var goUp = document.getElementById('goUp');
    if (!goUp) return;

    window.addEventListener('scroll', function() {
        if (window.pageYOffset > 300) {
            goUp.classList.add('visible');
        } else {
            goUp.classList.remove('visible');
        }
    });
})();