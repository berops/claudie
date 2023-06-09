<div style="display: flex; justify-content: center;">
    <form role="form" style="text-aling: center;" action="https://formspree.io/f/mzbqlkdj" method="POST">
        <label for="message">
            Your message:
        </label>
        <br>
        <textarea rows="10" name="message" id="message"
                style="padding: 8px; border: 1px solid #ccc; border-radius: 4px; width: 400px; margin: 1% 0 3% 0;"></textarea>
        <br>
        <button 
            type="submit" 
            style="padding: 8px 16px; width: 100%; background-color: #4CAF50; color: white; border: none; border-radius: 4px; cursor: pointer;">
            Send
        </button>
    </form>
</div>

<script>
    const mainHeader = document.querySelector('h1');
    mainHeader.style.textAlign = "center";
    mainHeader.style.margin = "0 0 0.75em";
</script>
