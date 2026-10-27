import { useEffect } from 'react';

const APP_NAME = 'Qadam';

/** The title of the open route (from its static data), set by the app root. */
let routeTitle: string | undefined;
/** Titles of mounted pages that know better (a project, a showcase page); the newest wins. */
const pageTitles: { title: string }[] = [];

function applyTitle() {
  document.title = pageTitles.at(-1)?.title ?? routeTitle ?? APP_NAME;
}

/** The route's own title, shown unless a page sets a more specific one. */
export function setRouteTitle(title: string | undefined): void {
  routeTitle = title;
  applyTitle();
}

function descriptionTag(): HTMLMetaElement {
  let tag = document.querySelector<HTMLMetaElement>('meta[name="description"]');
  if (!tag) {
    tag = document.createElement('meta');
    tag.name = 'description';
    document.head.append(tag);
  }
  return tag;
}

/**
 * The tab title and the description of a page. The server renders both for
 * crawlers (showcase SEO); this keeps them right while the visitor navigates
 * the app. The page's title wins over the route's while the page is mounted;
 * the description in place before comes back when the page is left.
 */
export function useDocumentMeta(title: string | undefined, description?: string): void {
  useEffect(() => {
    if (!title) return;
    const entry = { title };
    pageTitles.push(entry);
    applyTitle();
    const tag = description ? descriptionTag() : undefined;
    const previous = tag?.content;
    if (tag && description) tag.content = description;
    return () => {
      pageTitles.splice(pageTitles.indexOf(entry), 1);
      applyTitle();
      if (tag && previous !== undefined) tag.content = previous;
    };
  }, [title, description]);
}
