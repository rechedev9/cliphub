import { useLocation, useNavigate, useParams as useRouterParams, useSearchParams as useRouterSearchParams } from 'react-router-dom';

interface NavigateOptions {
  scroll?: boolean;
}

export function useRouter(): {
  push(href: string, options?: NavigateOptions): void;
  replace(href: string, options?: NavigateOptions): void;
  back(): void;
} {
  const navigate = useNavigate();
  return {
    push: (href) => navigate(href),
    replace: (href) => navigate(href, { replace: true }),
    back: () => navigate(-1),
  };
}

export function usePathname(): string {
  return useLocation().pathname;
}

export function useSearchParams(): URLSearchParams {
  return useRouterSearchParams()[0];
}

export function useParams(): Readonly<Record<string, string | undefined>> {
  return useRouterParams();
}
