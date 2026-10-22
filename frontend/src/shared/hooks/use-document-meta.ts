import { useEffect } from 'react';

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
 * the app, and puts the previous ones back when the page is left.
 */
export function useDocumentMeta(title: string | undefined, description?: string): void {
  useEffect(() => {
    if (!title) return;
    const tag = descriptionTag();
    const previous = { title: document.title, description: tag.content };
    document.title = title;
    if (description) tag.content = description;
    return () => {
      document.title = previous.title;
      tag.content = previous.description;
    };
  }, [title, description]);
}
