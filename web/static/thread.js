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
