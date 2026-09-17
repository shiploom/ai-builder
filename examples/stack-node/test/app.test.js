'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { TOKEN_TTL_S, makeToken, isExpired } = require('../app');

test('token has 128 bits', () => {
  assert.equal(makeToken().length, 32);
});

test('tokens differ', () => {
  assert.notEqual(makeToken(), makeToken());
});

test('fresh token valid', () => {
  assert.equal(isExpired(1000.0, 1000.0 + TOKEN_TTL_S - 1), false);
});

test('old token rejected', () => {
  assert.equal(isExpired(1000.0, 1000.0 + TOKEN_TTL_S + 1), true);
});
