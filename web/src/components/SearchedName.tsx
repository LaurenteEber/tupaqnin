import type { SearchedSource } from "../api/graph";
import { translate, type Lang } from "../i18n";
import { safeLink, searchedName } from "../provenance";

// A searched source (backlog-r3 3.5): title, year and link, the API's.
export function SearchedName({ lang, source }: { lang: Lang; source: SearchedSource }) {
  const link = safeLink(source.url);
  return (
    <>
      {searchedName(source)}
      {link && (
        <>
          {" "}
          <a href={link} target="_blank" rel="noopener noreferrer">
            {translate(lang, "provenance.open_link")}
          </a>
        </>
      )}
    </>
  );
}
