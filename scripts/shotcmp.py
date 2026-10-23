#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""多视口截图逐像素比对。

用法:
    python scripts/shotcmp.py <dirA> <dirB> [--out <diff_dir>]

对同名 PNG 逐张比对，输出:
  - 尺寸是否一致
  - 差异像素数量与差异区域包围盒
  - 差异区域的并排裁切图（写到 --out 目录，便于人工确认）

退出码: 0 全部一致；1 存在差异；2 参数/文件错误。
"""

import os
import sys

from PIL import Image, ImageChops


def compare(a_path, b_path, out_dir, name):
    a = Image.open(a_path).convert('RGBA')
    b = Image.open(b_path).convert('RGBA')
    if a.size != b.size:
        return {
            'name': name,
            'status': 'SIZE_DIFF',
            'a_size': a.size,
            'b_size': b.size,
        }

    # 注意：必须先去掉 alpha 通道再求差异。
    # Pillow 的 getbbox() 对带 alpha 的图像以 alpha 作为掩码，而
    # ImageChops.difference 的 alpha 恒为 0（|255-255|），直接用会永远
    # 得到"无差异"的错误结论。
    diff = ImageChops.difference(a.convert('RGB'), b.convert('RGB'))
    bbox = diff.getbbox()
    if bbox is None:
        return {'name': name, 'status': 'IDENTICAL', 'size': a.size}

    # 统计差异像素数（任一通道不同即计为差异像素）。
    gray = diff.convert('L')
    hist = gray.histogram()
    diff_px = sum(hist[1:])
    total = a.size[0] * a.size[1]
    ratio = diff_px / total if total else 0.0

    result = {
        'name': name,
        'status': 'DIFF',
        'size': a.size,
        'diff_px': diff_px,
        'diff_ratio': round(ratio, 6),
        'bbox': bbox,
    }

    if out_dir:
        os.makedirs(out_dir, exist_ok=True)
        pad = 12
        x0 = max(0, bbox[0] - pad)
        y0 = max(0, bbox[1] - pad)
        x1 = min(a.size[0], bbox[2] + pad)
        y1 = min(a.size[1], bbox[3] + pad)
        ca = a.crop((x0, y0, x1, y1))
        cb = b.crop((x0, y0, x1, y1))
        # 差异放大 3 倍，便于肉眼识别。
        scale = 3
        ca = ca.resize((ca.width * scale, ca.height * scale), Image.NEAREST)
        cb = cb.resize((cb.width * scale, cb.height * scale), Image.NEAREST)
        strip = Image.new('RGBA', (ca.width * 2 + 24, ca.height), (255, 0, 255, 255))
        strip.paste(ca, (0, 0))
        strip.paste(cb, (ca.width + 24, 0))
        strip.save(os.path.join(out_dir, name + '.diff.png'))
        result['diff_image'] = os.path.join(out_dir, name + '.diff.png')
    return result


def main(argv):
    if len(argv) < 3:
        sys.stderr.write(__doc__)
        return 2
    dir_a, dir_b = argv[1], argv[2]
    out_dir = None
    if '--out' in argv:
        out_dir = argv[argv.index('--out') + 1]

    if not os.path.isdir(dir_a) or not os.path.isdir(dir_b):
        sys.stderr.write('目录不存在: %s 或 %s\n' % (dir_a, dir_b))
        return 2

    names = sorted(f for f in os.listdir(dir_a) if f.endswith('.png'))
    if not names:
        sys.stderr.write('%s 下没有 PNG\n' % dir_a)
        return 2

    results = []
    for name in names:
        b_path = os.path.join(dir_b, name)
        if not os.path.exists(b_path):
            results.append({'name': name, 'status': 'MISSING_IN_B'})
            continue
        results.append(compare(os.path.join(dir_a, name), b_path, out_dir, name))

    width = max(len(r['name']) for r in results)
    same = 0
    for r in results:
        if r['status'] == 'IDENTICAL':
            same += 1
            line = '%-*s  IDENTICAL  %s' % (width, r['name'], r.get('size'))
        elif r['status'] == 'DIFF':
            line = '%-*s  DIFF       %s  diff_px=%d (%.4f%%) bbox=%s' % (
                width, r['name'], r.get('size'), r['diff_px'],
                r['diff_ratio'] * 100, r.get('bbox'))
        else:
            line = '%-*s  %s' % (width, r['name'], r['status'])
        print(line)

    print()
    print('合计 %d 张：完全一致 %d，存在差异 %d' % (
        len(results), same, sum(1 for r in results if r['status'] == 'DIFF')))
    return 0 if same == len(results) else 1


if __name__ == '__main__':
    sys.exit(main(sys.argv))
