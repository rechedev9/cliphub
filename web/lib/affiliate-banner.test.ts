import test from 'node:test';
import assert from 'node:assert/strict';
import {
  AFFILIATE_FAMILY,
  affiliateDisplayLabel,
  affiliateFamilyLabel,
  affiliatePlateFile,
  affiliateStyleLabel,
  effectiveAffiliateFamily,
  isAffiliateStyle,
  stylesForFamily,
} from './api/types.ts';
import {
  affiliatePlatesShareFile,
  persistAffiliateFamily,
  selectAffiliateFamily,
  selectAffiliateOff,
  selectAffiliateStyle,
} from './affiliate-banner.ts';

test('KEYDROP and CSGOSKINS do not share plates or brief labels', () => {
  assert.equal(affiliatePlateFile(AFFILIATE_FAMILY.keydrop, 'classic'), 'style-classic.png');
  assert.equal(affiliatePlateFile(AFFILIATE_FAMILY.csgoskins, 'classic'), 'csgoskins-classic.png');
  assert.equal(
    affiliatePlatesShareFile(AFFILIATE_FAMILY.keydrop, AFFILIATE_FAMILY.csgoskins, 'classic'),
    false,
  );
  assert.equal(affiliateFamilyLabel(AFFILIATE_FAMILY.keydrop, 'classic'), 'KeyDrop');
  assert.equal(affiliateFamilyLabel(AFFILIATE_FAMILY.csgoskins, 'classic'), 'CSGOSkins');
  assert.notEqual(
    affiliateFamilyLabel(AFFILIATE_FAMILY.keydrop, 'classic'),
    affiliateFamilyLabel(AFFILIATE_FAMILY.csgoskins, 'classic'),
  );
});

test('styles stay scoped to the selected family', () => {
  const keyDropIds = stylesForFamily(AFFILIATE_FAMILY.keydrop).map((entry) => entry.id);
  const skinsIds = stylesForFamily(AFFILIATE_FAMILY.csgoskins).map((entry) => entry.id);
  assert.deepEqual(keyDropIds, ['operator', 'classic', 'tigerr', 'jcorko']);
  assert.deepEqual(skinsIds, ['classic', 'operator']);
  assert.equal(isAffiliateStyle(AFFILIATE_FAMILY.keydrop, 'tigerr'), true);
  assert.equal(isAffiliateStyle(AFFILIATE_FAMILY.csgoskins, 'tigerr'), false);
  assert.equal(isAffiliateStyle(AFFILIATE_FAMILY.csgoskins, 'classic'), true);
  assert.equal(affiliateStyleLabel(AFFILIATE_FAMILY.csgoskins, 'classic'), 'Classic');
});

test('Sin banner turns the plate off and a later render keeps the chosen family', () => {
  const off = selectAffiliateOff();
  assert.deepEqual(off, { family: '', style: '' });
  assert.equal(persistAffiliateFamily(off.family, off.style), '');

  const keyDrop = selectAffiliateStyle('', 'tigerr');
  assert.deepEqual(keyDrop, { family: AFFILIATE_FAMILY.keydrop, style: 'tigerr' });
  assert.equal(persistAffiliateFamily(keyDrop.family, keyDrop.style), AFFILIATE_FAMILY.keydrop);
  assert.equal(affiliateDisplayLabel(keyDrop.family, keyDrop.style, 'zackcsgo'), 'CODE: ZACKCSGO');

  const skins = selectAffiliateFamily(keyDrop, AFFILIATE_FAMILY.csgoskins);
  assert.equal(skins.family, AFFILIATE_FAMILY.csgoskins);
  assert.equal(skins.style, 'classic');
  assert.equal(persistAffiliateFamily(skins.family, skins.style), AFFILIATE_FAMILY.csgoskins);
  assert.equal(isAffiliateStyle(skins.family, 'tigerr'), false);
  assert.equal(affiliateDisplayLabel(skins.family, skins.style, 'zackcsgo'), 'CODE: ZACKCSGO');
  assert.deepEqual(selectAffiliateOff(), { family: '', style: '' });
});

test('preview copy stays CODE and follows the family catalog prefix', () => {
  assert.equal(affiliateDisplayLabel(AFFILIATE_FAMILY.keydrop, 'tigerr', 'zackcsgo'), 'CODE: ZACKCSGO');
  assert.equal(affiliateDisplayLabel(AFFILIATE_FAMILY.keydrop, 'jcorko', 'zackcsgo'), 'CODIGO: ZACKCSGO');
  assert.equal(affiliateDisplayLabel(AFFILIATE_FAMILY.csgoskins, 'classic', 'skins99'), 'CODE: SKINS99');
  assert.equal(effectiveAffiliateFamily('', 'classic'), AFFILIATE_FAMILY.keydrop);
});
