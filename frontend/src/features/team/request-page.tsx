import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import { ArrowLeft } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  EmptyState,
  SkeletonCard,
} from '@/design-system';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';

import { teamRequestQuery, type TeamRequest } from './api';
import { RequestCard } from './request-card';
import { RequestDialog, RespondDialog, ResponsesList } from './request-dialogs';

/** One request for teammates; its author also sees the replies here. */
export function TeamRequestPage({ requestId }: { requestId: string }) {
  const { t } = useTranslation(['team', 'common']);
  const request = useQuery(teamRequestQuery(requestId));
  const [editing, setEditing] = useState<TeamRequest | null>(null);
  const [responding, setResponding] = useState<TeamRequest | null>(null);
  const notFound =
    request.error instanceof ApiError &&
    (request.error.status === 404 || request.error.status === 400);

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
      <Button asChild variant="ghost" size="sm" className="self-start">
        <Link to="/app/team">
          <ArrowLeft aria-hidden />
          {t('detail.back')}
        </Link>
      </Button>

      {request.isPending && <SkeletonCard />}
      {request.isError && (
        <EmptyState
          illustration={notFound ? 'search' : 'error'}
          title={notFound ? t('detail.notFound') : t('detail.failed')}
          description={notFound ? t('detail.notFoundText') : errorMessage(request.error)}
          action={
            <Button asChild variant="secondary">
              <Link to="/app/team">{t('detail.back')}</Link>
            </Button>
          }
        />
      )}
      {request.isSuccess && (
        <>
          <h1 className="sr-only">{request.data.title}</h1>
          <RequestCard
            request={request.data}
            onRespond={setResponding}
            onEdit={setEditing}
            onShowResponses={() => {
              document.getElementById('responses')?.scrollIntoView({ behavior: 'smooth' });
            }}
          />
          {request.data.is_mine && (
            <Card id="responses" className="scroll-mt-24">
              <CardHeader>
                <CardTitle>{t('responses.title')}</CardTitle>
              </CardHeader>
              <CardContent>
                <ResponsesList requestId={request.data.id} />
              </CardContent>
            </Card>
          )}
        </>
      )}

      <RequestDialog
        open={editing !== null}
        request={editing ?? undefined}
        onOpenChange={(open) => {
          if (!open) setEditing(null);
        }}
      />
      <RespondDialog
        request={responding}
        onOpenChange={(open) => {
          if (!open) setResponding(null);
        }}
      />
    </div>
  );
}

const requestRoute = getRouteApi('/app/team/$requestId');

export function TeamRequestRoute() {
  const { requestId } = requestRoute.useParams();
  return <TeamRequestPage key={requestId} requestId={requestId} />;
}
