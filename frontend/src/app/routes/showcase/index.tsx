import { createFileRoute } from '@tanstack/react-router';

import { CatalogPage } from '@/features/showcase/catalog-page';
import { parseShowcaseSearch } from '@/features/showcase/logic';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/showcase/')({
  validateSearch: parseShowcaseSearch,
  loader: () => loadNamespaces('showcase'),
  component: CatalogPage,
});
