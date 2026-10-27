import { z } from 'zod';

import { installZodLocalization } from './validation';

// Forms import zod from here: messages speak the interface language from the
// first schema on, and zod loads with the first form instead of the first page.
installZodLocalization();

export { z };
