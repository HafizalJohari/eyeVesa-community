(function() {
    var goUp = document.getElementById('goUp');
    var topbar = document.getElementById('topbar');

    window.addEventListener('scroll', function() {
        if (window.pageYOffset > 300) {
            goUp.style.display = 'block';
        } else {
            goUp.style.display = 'none';
        }
    });
})();