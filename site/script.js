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