(function() {
    var goUp = document.getElementById('goUp');
    if (goUp) {
        window.addEventListener('scroll', function() {
            if (window.pageYOffset > 300) {
                goUp.classList.add('visible');
            } else {
                goUp.classList.remove('visible');
            }
        });
    }

    var textElement = document.getElementById('typing-text');
    if (textElement) {
        var text = "<b>TL;DR</b><br><br>eyeVesa gives AI agents a cryptographic ID, enforces strict rules on what they can do (auto-allow, human-in-the-loop, or deny), and records every action in a tamper-proof audit log.<br><br><i>Think KYC and enterprise access control, but for AI agents.</i>";
        var i = 0;

        function typeWriter() {
            if (i < text.length) {
                var c = text.charAt(i);
                if (c === '<') {
                    var tagEnd = text.indexOf('>', i);
                    if (tagEnd !== -1) i = tagEnd;
                }
                textElement.innerHTML = text.substring(0, i + 1);
                i++;
                var delay = (c === '<') ? 0 : 15 + Math.random() * 20;
                setTimeout(typeWriter, delay);
            }
        }
        setTimeout(typeWriter, 800);
    }

    // Hamburger menu toggle
    var menuToggle = document.getElementById('menu-toggle');
    var nav = document.getElementById('nav');
    if (menuToggle && nav) {
        menuToggle.addEventListener('click', function(e) {
            e.stopPropagation();
            nav.classList.toggle('open');
            var isOpen = nav.classList.contains('open');
            menuToggle.textContent = isOpen ? 'Menu ▴' : 'Menu ▾';
        });
    }

    // Dropdown toggle on mobile
    var navLinks = document.querySelectorAll('.nav-item > .nav-link');
    navLinks.forEach(function(link) {
        link.addEventListener('click', function(e) {
            if (window.innerWidth <= 700 || link.getAttribute('href') === 'javascript:void(0)') {
                e.preventDefault();
                e.stopPropagation();
                var parent = link.parentElement;
                var isActive = parent.classList.contains('active');
                
                // Close all other active dropdowns
                document.querySelectorAll('.nav-item').forEach(function(item) {
                    item.classList.remove('active');
                });
                
                // Toggle current dropdown
                if (!isActive) {
                    parent.classList.add('active');
                }
            }
        });
    });

    // Close menu when clicking outside
    document.addEventListener('click', function() {
        if (nav && nav.classList.contains('open') && window.innerWidth <= 700) {
            nav.classList.remove('open');
            if (menuToggle) menuToggle.textContent = 'Menu ▾';
        }
        document.querySelectorAll('.nav-item').forEach(function(item) {
            item.classList.remove('active');
        });
    });

    // Close menu and dropdowns when a sub-item is clicked
    var dropdownItems = document.querySelectorAll('.dropdown-item a, .nav-item > a:not([href="javascript:void(0)"])');
    dropdownItems.forEach(function(item) {
        item.addEventListener('click', function() {
            if (nav && nav.classList.contains('open')) {
                nav.classList.remove('open');
                if (menuToggle) menuToggle.textContent = 'Menu ▾';
            }
            document.querySelectorAll('.nav-item').forEach(function(item) {
                item.classList.remove('active');
            });
        });
    });
    // Mouse tracking for interactive mesh gradient background
    var mouseX = window.innerWidth / 2;
    var mouseY = window.innerHeight / 2;
    var targetX = mouseX;
    var targetY = mouseY;
    
    document.addEventListener('mousemove', function(e) {
        targetX = e.clientX;
        targetY = e.clientY;
    });

    function updateMousePosition() {
        // Smooth lerp (linear interpolation) to make the motion extremely buttery and fluid
        mouseX += (targetX - mouseX) * 0.08;
        mouseY += (targetY - mouseY) * 0.08;
        document.documentElement.style.setProperty('--mouse-x', mouseX + 'px');
        document.documentElement.style.setProperty('--mouse-y', mouseY + 'px');
        requestAnimationFrame(updateMousePosition);
    }
    updateMousePosition();
})();

function copyCode(id) {
    var el = document.getElementById(id);
    if (!el) return;
    var text = el.textContent || el.innerText;
    if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).catch(function() {});
    } else {
        var ta = document.createElement('textarea');
        ta.value = text;
        ta.style.position = 'fixed';
        ta.style.left = '-9999px';
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
    }
    var btn = el.parentNode.querySelector('.copy-btn');
    if (btn) {
        var orig = btn.textContent;
        btn.textContent = '[Copied]';
        setTimeout(function() { btn.textContent = orig; }, 1500);
    }
}