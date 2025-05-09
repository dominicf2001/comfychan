const $ = (id) => document.getElementById(id);

function isOffScreen(el) {
    const rect = el.getBoundingClientRect();
    const vh = window.innerHeight || document.documentElement.clientHeight;
    const vw = window.innerWidth || document.documentElement.clientWidth;

    return (
        rect.bottom < 0 ||
        rect.top > vh ||
        rect.right < 0 ||
        rect.left > vw
    );
}

function smoothScrollTo(loc) {
    if (loc === "top") {
        window.scrollTo({ top: 0, behavior: 'smooth' });
    }
    else if (loc === "bottom") {
        window.scrollTo({ top: document.body.scrollHeight, behavior: 'smooth' });
    }
}

function isHttpWarningStatus(code) {
    const warningStatuses = [429, 400, 413, 403];
    return warningStatuses.includes(code);
}

function insertAfter(referenceNode, newNode) {
    referenceNode.parentNode.insertBefore(newNode, referenceNode.nextSibling);
}

function onReplyLinkClick(e) {
    const hoveringPost = document.getElementById("hoveringPost");
    hoveringPost?.remove();
}

function highlightPost(postId, e, status = true) {
    const posts = Array.from(document.querySelectorAll("article"));
    const postToHighlight = posts.find(p => +p.id.split("-")[1] === +postId);
    if (!postToHighlight) return;

    const isOp = postToHighlight.id === posts[0]?.id;
    if (isOffScreen(postToHighlight) || isOp) {
        if (e.type === "mouseleave") {
            const hoveringPost = document.querySelector("#hoveringPost")
            hoveringPost.remove();
        }
        else {
            const postCopy = postToHighlight.cloneNode(true);
            postCopy.style.position = "absolute";
            postCopy.id = "hoveringPost";

            if (isOp) {
                postCopy.classList.remove("post-op");
                postCopy.classList.add("post");
            }

            insertAfter(e.target, postCopy);
        }
    }
    else {
        if (status) {
            postToHighlight.classList.add("post-highlighted");
        }
        else {
            postToHighlight.classList.remove("post-highlighted");
        }
    }
}

function resizeCatalogPreviewImgs() {
    const value = $("selectResize").value;
    const postsImages = Array.from(document.querySelectorAll(".catalog-preview img"));
    for (const postImage of postsImages) {
        switch (value) {
            case "small":
                postImage.classList.remove("catalog-preview-img-large")
                postImage.classList.add("catalog-preview-img-small")
                break;
            case "large":
                postImage.classList.remove("catalog-preview-img-small")
                postImage.classList.add("catalog-preview-img-large")
                break;
        }
    }
}

function applyCatalogSearch() {
    const searchText = $("catalogSearch").value.toLowerCase();

    const posts = Array.from(document.querySelectorAll(".catalog-preview"));
    for (const post of posts) {
        const headerContent = post.querySelector("h1 a").textContent;
        const bodyContent = post.querySelector("p").textContent;

        const contentToSearch = (headerContent + "\n" + bodyContent).toLowerCase();
        if (contentToSearch.search(searchText) === -1) {
            post.style.display = "none";
        }
        else {
            post.style.display = "";
        }
    }
}

function applyCatalogSort(desc = true) {
    const sortBy = $("selectSortBy").value;

    const catalog = document.getElementById('catalog');
    const previews = Array.from(catalog.children);

    previews.sort((a, b) => {
        const pinDiff = (+b.dataset.pinned) - (+a.dataset.pinned);
        if (pinDiff !== 0) return pinDiff;

        const bumpDiff = (+a.dataset[sortBy]) - (+b.dataset[sortBy]);
        return desc ? -bumpDiff : bumpDiff;
    });

    const frag = document.createDocumentFragment();
    previews.forEach(el => frag.appendChild(el));
    catalog.appendChild(frag);
}

function currentBoardSlug() {
    return window.location.pathname.split("/")[1] || "";
}

function getCurrentDateISOString() {
    return (new Date()).toISOString().slice(0, 16)
}
