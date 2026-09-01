import {
  AFFILIATE_FAMILY,
  affiliatePlateFile,
  effectiveAffiliateFamily,
  isAffiliateStyle,
  isKeyDropStyle,
  normalizeAffiliateFamily,
  stylesForFamily,
  type AffiliateFamily,
  type KeyDropStyle,
} from './api/types.ts';

export type AffiliateBannerDraft = {
  family: AffiliateFamily | '';
  style: KeyDropStyle | '';
};

function catalogStyle(id: string): KeyDropStyle | '' {
  return isKeyDropStyle(id) ? id : '';
}

export function selectAffiliateOff(): AffiliateBannerDraft {
  return { family: '', style: '' };
}

export function selectAffiliateFamily(
  current: AffiliateBannerDraft,
  family: AffiliateFamily,
): AffiliateBannerDraft {
  const styles = stylesForFamily(family);
  const keep = current.style !== '' && styles.some((entry) => entry.id === current.style);
  return { family, style: keep ? catalogStyle(current.style) : catalogStyle(styles[0]?.id ?? '') };
}

export function selectAffiliateStyle(family: string, style: string): AffiliateBannerDraft {
  if (!style.trim()) return selectAffiliateOff();
  const fam = (normalizeAffiliateFamily(family) || AFFILIATE_FAMILY.keydrop) as AffiliateFamily;
  if (!isAffiliateStyle(fam, style)) {
    const fallback = stylesForFamily(fam)[0]?.id ?? '';
    return { family: fam, style: catalogStyle(fallback) };
  }
  return { family: fam, style: catalogStyle(style) };
}

export function persistAffiliateFamily(family: string, style: string): AffiliateFamily | '' {
  if (!style.trim()) return '';
  return effectiveAffiliateFamily(family, style);
}

export function affiliatePlatesShareFile(leftFamily: string, rightFamily: string, style: string): boolean {
  const left = affiliatePlateFile(leftFamily, style);
  const right = affiliatePlateFile(rightFamily, style);
  return left !== '' && left === right;
}
