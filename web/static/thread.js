function insertHeaderReplies() {
    const posts = Array.from(document.querySelectorAll("article"));
    const replies = {};

    for (const post of posts) {
        const postId = +post.id.split("-")[1];
        const postBody = post.querySelector("p").textContent;

        const replyLinkIds = [...postBody.matchAll(/(?:^|\s)>>(\d+)\b/g)].map(m => parseInt(m[1]));
        for (const replyLinkId of replyLinkIds) {
            replies[replyLinkId] = replies[replyLinkId] ?
                replies[replyLinkId].add(postId) :
                replies[replyLinkId] = new Set([postId]);
        }
    }

    for (const postId in replies) {
        const post = $(`post-${postId}`);
        if (!post) break;

        const postRepliesContainer = post.querySelector(".post-replies");

        const replyIds = Array.from(replies[postId]);
        for (const replyId of replyIds) {
            const replyLink = document.createElement("a");
            replyLink.textContent = `>>${replyId}`;
            replyLink.classList.add("reply-link-header");
            replyLink.setAttribute("href", `#post-${replyId}`);

            replyLink.onmouseover = (e) => highlightPost(+replyId, e);
            replyLink.onmouseleave = (e) => highlightPost(+replyId, e, false);
            replyLink.onclick = onReplyLinkClick;

            postRepliesContainer.append(replyLink);
        }
    }
}

function togglePostFile(el) {
    function makeOpaqueUntilReady(el, readyEvt) {
        el.classList.add('post-file-loading');
        el.addEventListener(readyEvt, () => el.classList.remove('post-file-loading'), { once: true });
    }

    const imgEl = el.parentElement.parentElement.querySelector("img");
    const isVideo = imgEl.dataset.full.endsWith(".mp4") || imgEl.dataset.full.endsWith(".webm") || imgEl.dataset.full.endsWith(".ogg");

    if (isVideo) {
        const vidEl = imgEl.parentElement.querySelector("video");

        const closeVidBtn = imgEl.parentElement.querySelector(".link-button");
        if (vidEl.style.display === "none") {
            vidEl.src = "/media/posts/full/" + imgEl.dataset.full;
            makeOpaqueUntilReady(vidEl, 'canplay');
            vidEl.style.display = "";
            imgEl.style.display = "none";
            closeVidBtn.style.display = "";
        }
        else {
            vidEl.src = "";
            vidEl.style.display = "none";
            imgEl.style.display = "";
            closeVidBtn.style.display = "none";
        }
    }
    else {
        if (imgEl.classList.contains("post-img-full")) {
            imgEl.classList.remove("post-img-full");
            imgEl.src = "/media/posts/thumb/" + imgEl.dataset.thumb;
        }
        else {
            imgEl.classList.add("post-img-full");
            imgEl.src = "/media/posts/full/" + imgEl.dataset.full;
            if (!imgEl.complete)
                makeOpaqueUntilReady(imgEl, 'load');
        }
    }
}

function initializePosts() {
    // set dates to correct timezone
    document.querySelectorAll('.post-datetime').forEach(el => {
        const utcDateStr = el.getAttribute('data-utc');
        const localDate = new Date(utcDateStr);

        const hours = localDate.getHours() % 12 || 12;
        const minutes = String(localDate.getMinutes()).padStart(2, '0');
        const ampm = localDate.getHours() >= 12 ? 'PM' : 'AM';

        const month = String(localDate.getMonth() + 1).padStart(2, '0');
        const day = String(localDate.getDate()).padStart(2, '0');
        const year = localDate.getFullYear();

        el.textContent = `${month}/${day}/${year} ${hours}:${minutes} ${ampm}`;
    });

    insertHeaderReplies()
}

initializePosts();
