import { inlineSpans, parseBlocks } from './markdown';

function Inline({ text }: { text: string }) {
  return (
    <>
      {inlineSpans(text).map((span, index) =>
        span.bold ? (
          <strong key={index} className="font-semibold">
            {span.text}
          </strong>
        ) : (
          <span key={index}>{span.text}</span>
        ),
      )}
    </>
  );
}

/** Paragraphs, bullet and numbered lists and bold text as React elements (never HTML). */
export function LightMarkdown({ text }: { text: string }) {
  return (
    <>
      {parseBlocks(text).map((block, index) => {
        if (block.kind === 'paragraph') {
          return (
            <p key={index} className="whitespace-pre-line">
              <Inline text={block.text} />
            </p>
          );
        }
        const items = block.items.map((item, i) => (
          <li key={i}>
            <Inline text={item} />
          </li>
        ));
        return block.ordered ? (
          <ol key={index} className="list-decimal space-y-1 pl-5">
            {items}
          </ol>
        ) : (
          <ul key={index} className="list-disc space-y-1 pl-5">
            {items}
          </ul>
        );
      })}
    </>
  );
}
