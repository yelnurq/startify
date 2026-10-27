import { createFileRoute, redirect } from '@tanstack/react-router';

// E-mails about replies link here; the request lives in the signed-in area.
export const Route = createFileRoute('/team/$requestId')({
  beforeLoad: ({ params }) => {
    throw redirect({ to: '/app/team/$requestId', params, replace: true });
  },
});
