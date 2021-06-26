'use strict';

module.exports = function(environment) {
  let ENV = {
    modulePrefix: 'open-zano-pool',
    environment: environment,
    rootURL: '/',
    locationType: 'hash',
    EmberENV: {
      FEATURES: {
        // Here you can enable experimental features on an ember canary build
        // e.g. 'with-controller': true
      }
    },

    APP: {
      // API host and port
      ApiUrl: 'http://zano.waietng-pool.us:21111/',

      // HTTP mining endpoint
      HttpHost: 'http://zano.waietng-pool.us',
      HttpPort: 11112,

      // Stratum mining endpoint
      StratumHost: 'zano.waietng-pool.us',
      StratumPort: 12111,

      // Fee and payout details
      PoolFee: '5%',
      PayoutThreshold: '0.5 Zano',

      // For network hashrate (change for your favourite fork)
      BlockTime: 33.0
    }
  };

  if (environment === 'development') {
    /* Override ApiUrl just for development, while you are customizing
      frontend markup and css theme on your workstation.
    */
    //ENV.APP.ApiUrl = 'http://localhost:8081/'
     ENV.APP.LOG_RESOLVER = true;
     ENV.APP.LOG_ACTIVE_GENERATION = true;
     ENV.APP.LOG_TRANSITIONS = true;
     ENV.APP.LOG_TRANSITIONS_INTERNAL = true;
     ENV.APP.LOG_VIEW_LOOKUPS = true;
  }

  if (environment === 'test') {
    // Testem prefers this...
    ENV.locationType = 'none';

    // keep test console output quieter
    ENV.APP.LOG_ACTIVE_GENERATION = false;
    ENV.APP.LOG_VIEW_LOOKUPS = false;

    ENV.APP.rootElement = '#ember-testing';
  }

  if (environment === 'production') {

  }

  return ENV;
};
