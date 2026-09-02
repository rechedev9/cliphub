import type { AnchorHTMLAttributes, ReactElement, ReactNode } from 'react';
import { Link as RouterLink } from 'react-router-dom';

interface LinkProps extends Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> {
  href: string;
  children?: ReactNode;
}

export default function Link({ href, children, ...props }: LinkProps): ReactElement {
  if (/^(?:[a-z]+:)?\/\//i.test(href)) {
    return <a href={href} {...props}>{children}</a>;
  }
  return <RouterLink to={href} {...props}>{children}</RouterLink>;
}

export function useLinkStatus(): { pending: false } {
  return { pending: false };
}
