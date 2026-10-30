#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
从 ISOGG Y-DNA 树 CSV（isogg_haplogroups_2019-2020.csv）生成 ydna.txt 和 ydna_alias.txt。

用法：
  1. 把 ISOGG Y-DNA 树导出 CSV 放到本目录（tools/），名为
     isogg_haplogroups_2019-2020.csv（列需含 id/lineage/snps）。
  2. 运行 python3 import_ydna.py，生成 backend/internal/handler/ydna.txt 与
     ydna_alias.txt。

数据源：ISOGG Y-DNA Haplogroup Tree 2019-2020，约 9.6 万节点。
更新方式：ISOGG 发布新版本 CSV 时，替换文件重跑本脚本即可。
"""
import csv


def canon_y(s):
    """与后端 canonicalHaplo 的 Y-DNA 规范化一致：首字母大写、其余小写。"""
    if not s:
        return s
    return s[0].upper() + s[1:].lower()


def load_rows(path):
    with open(path, encoding='utf-8-sig') as f:
        rows = list(csv.DictReader(f))
    return [r for r in rows if '==' not in r['id']]


def build_parent(rows):
    """从 lineage（祖先链，> 分隔）推导父子关系，前缀补跨页断链，顶层挂 root。"""
    parent = {}
    all_ids = set()
    for r in rows:
        parts = [canon_y(p) for p in r['lineage'].split('>') if p.strip()]
        all_ids.update(parts)
        for i in range(1, len(parts)):
            parent[parts[i]] = parts[i - 1]

    tops = set('ABCDEFGHIJKLMNOPQRST')
    broken = []
    for r in rows:
        cid = canon_y(r['id'])
        if cid not in parent:
            if cid.rstrip('~') in tops and len(cid.rstrip('~')) == 1:
                parent[cid] = ''  # 顶层挂 root
            else:
                broken.append(cid)

    for b in broken:  # 前缀补断链
        base = b.split('-')[0]
        found = None
        for k in range(len(base) - 1, 0, -1):
            if base[:k] in all_ids:
                found = base[:k]
                break
        if not found and base in all_ids and base != b:
            found = base
        if found:
            parent[b] = found

    all_nodes = set(parent) | all_ids
    return {c: p for c, p in parent.items() if p == '' or p in all_nodes}


def build_alias(rows, tree_nodes):
    """从 snps 第一个位点提取「位点 -> 单倍群名」，生成纯位点 + 字母-位点两种别名。"""
    snp2node = {}
    for r in rows:
        snps = r['snps'].split()
        if not snps:
            continue
        first = snps[0].split('/')[0]
        if first and first[0] in 'MFPSZ' and len(first) <= 8 and first.isalnum():
            node = canon_y(r['id'])
            snp2node.setdefault(first, node)

    alias = {}
    for snp, node in snp2node.items():
        alias[snp] = node
        letter = node[0].upper()
        if letter in 'ABCDEFGHIJKLMNOPQRST':
            alias[letter + '-' + snp] = node
    return {k: v for k, v in alias.items() if v in tree_nodes}


def main():
    rows = load_rows('isogg_haplogroups_2019-2020.csv')
    parent = build_parent(rows)
    alias = build_alias(rows, set(parent))

    out = '../backend/internal/handler/'
    with open(out + 'ydna.txt', 'w', encoding='utf-8') as f:
        f.write('\n'.join('%s\t%s' % (c, parent[c]) for c in sorted(parent)) + '\n')
    with open(out + 'ydna_alias.txt', 'w', encoding='utf-8') as f:
        f.write('\n'.join('%s\t%s' % (k, alias[k]) for k in sorted(alias)) + '\n')
    print('生成 ydna.txt：%d 父子关系；ydna_alias.txt：%d 别名' % (len(parent), len(alias)))


if __name__ == '__main__':
    main()
