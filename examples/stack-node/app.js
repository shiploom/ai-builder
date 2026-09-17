'use strict';

const crypto = require('node:crypto');

const TOKEN_TTL_S = 15 * 60;

function makeToken() {
  return crypto.randomBytes(16).toString('hex');
}

function isExpired(issuedAtS, nowS = Date.now() / 1000) {
  return (nowS - issuedAtS) > TOKEN_TTL_S;
}

module.exports = { TOKEN_TTL_S, makeToken, isExpired };
